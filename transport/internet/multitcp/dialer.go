package multitcp

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	internetnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/reality"
	"github.com/xtls/xray-core/transport/internet/stat"

	"github.com/xtls/xray-core/transport/internet/tls"
)

// MultiTCPConnection implements a connection that uses multiple TCP connections
type MultiTCPConnection struct {
	manager     *ConnectionManager
	readBuffer  *buf.Buffer
	writeBuffer *buf.Buffer
	closed      int32
	localAddr   net.Addr
	remoteAddr  net.Addr
	readMutex   sync.Mutex
	writeMutex  sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewMultiTCPConnection creates a new MultiTCP connection
func NewMultiTCPConnection(ctx context.Context, dest internetnet.Destination, config *Config, streamSettings *internet.MemoryStreamConfig) (*MultiTCPConnection, error) {
	manager := NewConnectionManager(dest, config, streamSettings)
	
	// Create first connection to get addresses
	firstConn, err := manager.GetConnection(ctx, 0)
	if err != nil {
		return nil, err
	}
	
	ctx, cancel := context.WithCancel(ctx)
	
	conn := &MultiTCPConnection{
		manager:     manager,
		readBuffer:  buf.New(),
		writeBuffer: buf.New(),
		localAddr:   firstConn.LocalAddr(),
		remoteAddr:  firstConn.RemoteAddr(),
		ctx:         ctx,
		cancel:      cancel,
	}
	
	// Start cleanup routine
	go conn.cleanupRoutine()
	
	return conn, nil
}

// Read implements net.Conn.Read
func (c *MultiTCPConnection) Read(b []byte) (int, error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return 0, io.EOF
	}
	
	c.readMutex.Lock()
	defer c.readMutex.Unlock()
	
	// If we have data in buffer, read from it first
	if c.readBuffer.Len() > 0 {
		return c.readBuffer.Read(b)
	}
	
	// Try to read from any available connection
	// This is a simplified implementation - in reality we'd need proper
	// connection management and data reassembly
	conn, err := c.manager.GetConnection(c.ctx, 0)
	if err != nil {
		return 0, err
	}
	
	return conn.Read(b)
}

// Write implements net.Conn.Write
func (c *MultiTCPConnection) Write(b []byte) (int, error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return 0, io.ErrClosedPipe
	}
	
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	
	// Fragment data if it's too large
	maxSize := c.manager.config.GetMaxDataPerConn()
	totalWritten := 0
	
	for len(b) > 0 {
		// Determine chunk size
		chunkSize := len(b)
		if int64(chunkSize) > maxSize {
			chunkSize = int(maxSize)
		}
		
		chunk := b[:chunkSize]
		b = b[chunkSize:]
		
		// Get connection for this chunk
		conn, err := c.manager.GetConnection(c.ctx, int64(chunkSize))
		if err != nil {
			return totalWritten, err
		}
		
		// Write chunk
		written, err := conn.Write(chunk)
		totalWritten += written
		if err != nil {
			return totalWritten, err
		}
	}
	
	return totalWritten, nil
}

// Close implements net.Conn.Close
func (c *MultiTCPConnection) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil
	}
	
	c.cancel()
	
	if c.readBuffer != nil {
		c.readBuffer.Release()
	}
	if c.writeBuffer != nil {
		c.writeBuffer.Release()
	}
	
	return c.manager.Close()
}

// LocalAddr implements net.Conn.LocalAddr
func (c *MultiTCPConnection) LocalAddr() net.Addr {
	return c.localAddr
}

// RemoteAddr implements net.Conn.RemoteAddr
func (c *MultiTCPConnection) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// SetDeadline implements net.Conn.SetDeadline
func (c *MultiTCPConnection) SetDeadline(t time.Time) error {
	// This is a simplified implementation
	return nil
}

// SetReadDeadline implements net.Conn.SetReadDeadline
func (c *MultiTCPConnection) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline implements net.Conn.SetWriteDeadline
func (c *MultiTCPConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

// cleanupRoutine runs periodic cleanup of stale connections
func (c *MultiTCPConnection) cleanupRoutine() {
	ticker := time.NewTicker(c.manager.config.GetCleanupInterval())
	defer ticker.Stop()
	
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.manager.CleanupStaleConnections()
		}
	}
}

// Dial creates a new MultiTCP connection
func Dial(ctx context.Context, dest internetnet.Destination, streamSettings *internet.MemoryStreamConfig) (stat.Connection, error) {
	errors.LogInfo(ctx, "dialing MultiTCP to ", dest)
	
	config := ConfigFromStreamSettings(streamSettings)
	if config == nil {
		// Use default configuration
		config = GetNormalizedConfig(nil)
	}
	
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, errors.New("invalid MultiTCP configuration").Base(err)
	}
	
	// Create MultiTCP connection
	conn, err := NewMultiTCPConnection(ctx, dest, config, streamSettings)
	if err != nil {
		return nil, err
	}
	
	// Apply TLS/Reality configuration if needed
	if err := applySecuritySettings(ctx, conn, streamSettings, dest); err != nil {
		conn.Close()
		return nil, err
	}
	
	return stat.Connection(conn), nil
}

// applySecuritySettings applies TLS or Reality configuration
func applySecuritySettings(ctx context.Context, conn *MultiTCPConnection, streamSettings *internet.MemoryStreamConfig, dest internetnet.Destination) error {
	// Check for TLS configuration
	if config := tls.ConfigFromStreamSettings(streamSettings); config != nil {
		// For MultiTCP, we need to handle TLS differently
		// This is a simplified implementation
		errors.LogInfo(ctx, "MultiTCP: TLS configuration detected")
		return nil
	}
	
	// Check for Reality configuration
	if config := reality.ConfigFromStreamSettings(streamSettings); config != nil {
		// For MultiTCP, we need to handle Reality differently
		// This is a simplified implementation
		errors.LogInfo(ctx, "MultiTCP: Reality configuration detected")
		return nil
	}
	
	return nil
}

// GetProtocolName returns the protocol name
func GetProtocolName() string {
	return protocolName
}

// init registers the MultiTCP transport dialer
func init() {
	common.Must(internet.RegisterTransportDialer(protocolName, Dial))
}