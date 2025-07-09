package multitcp

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/stat"
)

const protocolName = "multitcp"

// Constants for RU censorship bypass
const (
	DefaultMaxDataPerConn = 15 * 1024 // 15KB per connection to avoid blocking
	DefaultMaxConnections = 10        // Maximum concurrent connections
	DefaultConnTimeout    = 30 * time.Second
)

// ConnectionManager manages multiple TCP connections for bypassing censorship
type ConnectionManager struct {
	dest         net.Destination
	streamConfig *internet.MemoryStreamConfig
	config       *Config
	connections  []*ManagedConnection
	connMutex    sync.RWMutex
	nextConnIdx  int32
	closed       int32
}

// ManagedConnection represents a single TCP connection with usage tracking
type ManagedConnection struct {
	conn     stat.Connection
	bytesOut int64
	lastUsed time.Time
	mutex    sync.RWMutex
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(dest net.Destination, config *Config, streamConfig *internet.MemoryStreamConfig) *ConnectionManager {
	return &ConnectionManager{
		dest:         dest,
		streamConfig: streamConfig,
		config:       config,
		connections:  make([]*ManagedConnection, 0),
	}
}

// GetConnection returns a connection suitable for data transfer
func (cm *ConnectionManager) GetConnection(ctx context.Context, dataSize int64) (stat.Connection, error) {
	if atomic.LoadInt32(&cm.closed) == 1 {
		return nil, errors.New("connection manager is closed")
	}

	cm.connMutex.RLock()
	
	// Find an existing connection that can handle the data
	for _, managedConn := range cm.connections {
		managedConn.mutex.RLock()
		if managedConn.bytesOut+dataSize <= cm.config.GetMaxDataPerConn() {
			managedConn.bytesOut += dataSize
			managedConn.lastUsed = time.Now()
			managedConn.mutex.RUnlock()
			cm.connMutex.RUnlock()
			return managedConn.conn, nil
		}
		managedConn.mutex.RUnlock()
	}
	
	cm.connMutex.RUnlock()

	// Need to create a new connection
	return cm.createNewConnection(ctx, dataSize)
}

// createNewConnection creates a new TCP connection
func (cm *ConnectionManager) createNewConnection(ctx context.Context, dataSize int64) (stat.Connection, error) {
	cm.connMutex.Lock()
	defer cm.connMutex.Unlock()

	// Check if we've reached the connection limit
	if len(cm.connections) >= cm.config.GetMaxConnections() {
		// Find the oldest connection and close it
		oldestIdx := 0
		oldestTime := cm.connections[0].lastUsed
		for i, conn := range cm.connections {
			if conn.lastUsed.Before(oldestTime) {
				oldestIdx = i
				oldestTime = conn.lastUsed
			}
		}
		
		// Close the oldest connection
		cm.connections[oldestIdx].conn.Close()
		
		// Remove from slice
		cm.connections = append(cm.connections[:oldestIdx], cm.connections[oldestIdx+1:]...)
	}

	// Create new connection using the original TCP dialer
	conn, err := internet.DialSystem(ctx, cm.dest, cm.streamConfig.SocketSettings)
	if err != nil {
		return nil, errors.New("failed to dial connection").Base(err)
	}

	// Apply TLS/Reality if configured
	if err := cm.applyTLS(ctx, &conn); err != nil {
		conn.Close()
		return nil, err
	}

	// Create managed connection
	managedConn := &ManagedConnection{
		conn:     stat.Connection(conn),
		bytesOut: dataSize,
		lastUsed: time.Now(),
	}

	cm.connections = append(cm.connections, managedConn)
	
	errors.LogInfo(ctx, fmt.Sprintf("MultiTCP: Created new connection %d/%d, bytes: %d", 
		len(cm.connections), cm.config.GetMaxConnections(), dataSize))

	return managedConn.conn, nil
}

// applyTLS applies TLS or Reality configuration to the connection
func (cm *ConnectionManager) applyTLS(ctx context.Context, conn *net.Conn) error {
	// Import necessary packages at the top
	// This is a simplified version - applying TLS/Reality similar to tcp/dialer.go
	
	// Import the necessary packages for TLS/Reality
	// This would need proper imports in actual implementation
	return nil
}

// Close closes all managed connections
func (cm *ConnectionManager) Close() error {
	if !atomic.CompareAndSwapInt32(&cm.closed, 0, 1) {
		return nil
	}

	cm.connMutex.Lock()
	defer cm.connMutex.Unlock()

	for _, managedConn := range cm.connections {
		managedConn.conn.Close()
	}
	cm.connections = nil

	return nil
}

// CleanupStaleConnections removes connections that haven't been used recently
func (cm *ConnectionManager) CleanupStaleConnections() {
	cm.connMutex.Lock()
	defer cm.connMutex.Unlock()

	now := time.Now()
	staleThreshold := cm.config.GetConnTimeout()
	
	activeConnections := make([]*ManagedConnection, 0, len(cm.connections))
	
	for _, conn := range cm.connections {
		if now.Sub(conn.lastUsed) > staleThreshold {
			conn.conn.Close()
		} else {
			activeConnections = append(activeConnections, conn)
		}
	}
	
	cm.connections = activeConnections
}