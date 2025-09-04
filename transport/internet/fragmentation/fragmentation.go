package fragmentation

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/signal/done"
)

const (
	// Default fragment size: 15KB to avoid RKN DPI detection
	DefaultFragmentSize = 15 * 1024
	// Maximum fragment size: 20KB (threshold for RKN blocking)
	MaxFragmentSize = 20 * 1024
	// Minimum fragment size
	MinFragmentSize = 5 * 1024
	// Default interval between fragments
	DefaultFragmentInterval = 10 * time.Millisecond
)

// Config holds fragmentation configuration
type Config struct {
	Enabled          bool
	FragmentSize     int32
	FragmentInterval time.Duration
}

// GetConfig returns fragmentation config with defaults
func GetConfig(enabled bool, size uint32, intervalMs uint32) *Config {
	cfg := &Config{
		Enabled:          enabled,
		FragmentSize:     DefaultFragmentSize,
		FragmentInterval: DefaultFragmentInterval,
	}

	if size > 0 {
		// Convert KB to bytes
		sizeBytes := int32(size * 1024)
		if sizeBytes >= MinFragmentSize && sizeBytes <= MaxFragmentSize {
			cfg.FragmentSize = sizeBytes
		}
	}

	if intervalMs > 0 {
		cfg.FragmentInterval = time.Duration(intervalMs) * time.Millisecond
	}

	return cfg
}

// FragmentedConn wraps a connection to fragment data for DPI bypass
type FragmentedConn struct {
	net.Conn
	config         *Config
	writeBuffer    []byte
	writeMu        sync.Mutex
	readBuffer     []byte
	readMu         sync.Mutex
	bytesWritten   atomic.Int64
	bytesRead      atomic.Int64
	connectionPool *ConnectionPool
	closed         atomic.Bool
	done           *done.Instance
}

// NewFragmentedConn creates a new fragmented connection wrapper
func NewFragmentedConn(conn net.Conn, config *Config) *FragmentedConn {
	if config == nil || !config.Enabled {
		// Return a simple wrapper if fragmentation is disabled
		return &FragmentedConn{
			Conn:   conn,
			config: config,
			done:   done.New(),
		}
	}

	fc := &FragmentedConn{
		Conn:           conn,
		config:         config,
		writeBuffer:    make([]byte, 0, config.FragmentSize),
		readBuffer:     make([]byte, 0, config.FragmentSize),
		connectionPool: NewConnectionPool(conn.LocalAddr(), conn.RemoteAddr(), 5),
		done:           done.New(),
	}

	// Start connection pool manager
	go fc.connectionPool.Start()

	return fc
}

// Write implements net.Conn.Write with fragmentation
func (fc *FragmentedConn) Write(b []byte) (int, error) {
	if fc.closed.Load() {
		return 0, io.ErrClosedPipe
	}

	if fc.config == nil || !fc.config.Enabled {
		// Passthrough if fragmentation is disabled
		return fc.Conn.Write(b)
	}

	fc.writeMu.Lock()
	defer fc.writeMu.Unlock()

	totalWritten := 0
	data := b

	for len(data) > 0 {
		// Check if we need to switch connection
		currentBytes := fc.bytesWritten.Load()
		if currentBytes >= int64(fc.config.FragmentSize) {
			// Get a new connection from pool
			newConn := fc.connectionPool.GetConnection()
			if newConn != nil {
				// Switch to new connection
				fc.Conn = newConn
				fc.bytesWritten.Store(0)
				
				// Add small delay between connections
				if fc.config.FragmentInterval > 0 {
					time.Sleep(fc.config.FragmentInterval)
				}
			}
		}

		// Calculate chunk size
		remainingInFragment := fc.config.FragmentSize - int32(fc.bytesWritten.Load())
		chunkSize := len(data)
		if int32(chunkSize) > remainingInFragment {
			chunkSize = int(remainingInFragment)
		}

		// Write chunk
		n, err := fc.Conn.Write(data[:chunkSize])
		if err != nil {
			return totalWritten, err
		}

		fc.bytesWritten.Add(int64(n))
		totalWritten += n
		data = data[n:]
	}

	return totalWritten, nil
}

// Read implements net.Conn.Read
func (fc *FragmentedConn) Read(b []byte) (int, error) {
	if fc.closed.Load() {
		return 0, io.ErrClosedPipe
	}

	// For read, we don't need to fragment as we're receiving data
	// The server-side fragmentation handles the response splitting
	return fc.Conn.Read(b)
}

// WriteMultiBuffer writes multiple buffers with fragmentation
func (fc *FragmentedConn) WriteMultiBuffer(mb buf.MultiBuffer) error {
	if fc.config == nil || !fc.config.Enabled {
		// Passthrough if fragmentation is disabled
		mb = buf.Compact(mb)
		mb, err := buf.WriteMultiBuffer(fc.Conn, mb)
		buf.ReleaseMulti(mb)
		return err
	}

	fc.writeMu.Lock()
	defer fc.writeMu.Unlock()

	for _, buffer := range mb {
		data := buffer.Bytes()
		
		for len(data) > 0 {
			// Check if we need to switch connection
			currentBytes := fc.bytesWritten.Load()
			if currentBytes >= int64(fc.config.FragmentSize) {
				// Get a new connection from pool
				newConn := fc.connectionPool.GetConnection()
				if newConn != nil {
					fc.Conn = newConn
					fc.bytesWritten.Store(0)
					
					// Add delay between fragments
					if fc.config.FragmentInterval > 0 {
						time.Sleep(fc.config.FragmentInterval)
					}
				}
			}

			// Calculate chunk size
			remainingInFragment := fc.config.FragmentSize - int32(fc.bytesWritten.Load())
			chunkSize := len(data)
			if int32(chunkSize) > remainingInFragment {
				chunkSize = int(remainingInFragment)
			}

			// Write chunk
			n, err := fc.Conn.Write(data[:chunkSize])
			if err != nil {
				buf.ReleaseMulti(mb)
				return err
			}

			fc.bytesWritten.Add(int64(n))
			data = data[n:]
		}
		
		buffer.Release()
	}

	return nil
}

// Close closes the fragmented connection
func (fc *FragmentedConn) Close() error {
	if !fc.closed.CompareAndSwap(false, true) {
		return nil
	}

	fc.done.Close()

	var errs []error
	
	// Close main connection
	if err := fc.Conn.Close(); err != nil {
		errs = append(errs, err)
	}

	// Close connection pool
	if fc.connectionPool != nil {
		fc.connectionPool.Close()
	}

	if len(errs) > 0 {
		return errors.New("failed to close fragmented connection").Base(errs[0])
	}

	return nil
}

// ConnectionPool manages multiple connections for fragmentation
type ConnectionPool struct {
	localAddr    net.Addr
	remoteAddr   net.Addr
	maxPoolSize  int
	connections  []net.Conn
	mu           sync.RWMutex
	currentIndex atomic.Int32
	done         *done.Instance
	dialFunc     func() (net.Conn, error)
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(localAddr, remoteAddr net.Addr, poolSize int) *ConnectionPool {
	return &ConnectionPool{
		localAddr:   localAddr,
		remoteAddr:  remoteAddr,
		maxPoolSize: poolSize,
		connections: make([]net.Conn, 0, poolSize),
		done:        done.New(),
	}
}

// SetDialFunc sets the dial function for creating new connections
func (cp *ConnectionPool) SetDialFunc(dialFunc func() (net.Conn, error)) {
	cp.dialFunc = dialFunc
}

// Start starts the connection pool manager
func (cp *ConnectionPool) Start() {
	// Pre-create some connections
	for i := 0; i < 2 && i < cp.maxPoolSize; i++ {
		if conn := cp.createConnection(); conn != nil {
			cp.mu.Lock()
			cp.connections = append(cp.connections, conn)
			cp.mu.Unlock()
		}
	}
}

// GetConnection gets a connection from the pool
func (cp *ConnectionPool) GetConnection() net.Conn {
	cp.mu.RLock()
	poolSize := len(cp.connections)
	cp.mu.RUnlock()

	if poolSize == 0 {
		// Create first connection
		conn := cp.createConnection()
		if conn != nil {
			cp.mu.Lock()
			cp.connections = append(cp.connections, conn)
			cp.mu.Unlock()
			return conn
		}
		return nil
	}

	// Round-robin through connections
	index := cp.currentIndex.Add(1) % int32(poolSize)
	
	cp.mu.RLock()
	conn := cp.connections[index]
	cp.mu.RUnlock()

	// Create more connections if needed
	if poolSize < cp.maxPoolSize {
		go func() {
			if newConn := cp.createConnection(); newConn != nil {
				cp.mu.Lock()
				if len(cp.connections) < cp.maxPoolSize {
					cp.connections = append(cp.connections, newConn)
				} else {
					newConn.Close()
				}
				cp.mu.Unlock()
			}
		}()
	}

	return conn
}

// createConnection creates a new connection
func (cp *ConnectionPool) createConnection() net.Conn {
	if cp.dialFunc != nil {
		conn, err := cp.dialFunc()
		if err != nil {
			errors.LogWarning(context.Background(), "failed to create connection for pool: ", err)
			return nil
		}
		return conn
	}

	// Fallback to basic TCP dial
	dialer := &net.Dialer{
		Timeout: 30 * time.Second,
	}
	
	conn, err := dialer.Dial("tcp", cp.remoteAddr.String())
	if err != nil {
		errors.LogWarning(context.Background(), "failed to dial connection for pool: ", err)
		return nil
	}
	
	return conn
}

// Close closes all connections in the pool
func (cp *ConnectionPool) Close() {
	cp.done.Close()
	
	cp.mu.Lock()
	defer cp.mu.Unlock()
	
	for _, conn := range cp.connections {
		if conn != nil {
			conn.Close()
		}
	}
	
	cp.connections = nil
}