package websocket

import (
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/serial"
)

var _ buf.Writer = (*connection)(nil)

// connection is a wrapper for net.Conn over WebSocket connection.
// remoteAddr is used to pass "virtual" remote IP addresses in X-Forwarded-For.
// so we shouldn't directly read it form conn.
type connection struct {
	conn              *websocket.Conn
	reader            io.Reader
	remoteAddr        net.Addr
	fragmentConfig    *FragmentConfig
	bytesWritten      atomic.Int64
	connectionPool    []*websocket.Conn
	poolMu            sync.RWMutex
	currentConnIndex  atomic.Int32
}

// FragmentConfig holds fragmentation settings for DPI bypass
type FragmentConfig struct {
	Enabled          bool
	FragmentSize     int64  // Size in bytes
	FragmentInterval time.Duration
}

func NewConnection(conn *websocket.Conn, remoteAddr net.Addr, extraReader io.Reader, heartbeatPeriod uint32) *connection {
	return NewConnectionWithFragmentation(conn, remoteAddr, extraReader, heartbeatPeriod, nil)
}

// NewConnectionWithFragmentation creates a connection with optional fragmentation support
func NewConnectionWithFragmentation(conn *websocket.Conn, remoteAddr net.Addr, extraReader io.Reader, heartbeatPeriod uint32, fragmentConfig *FragmentConfig) *connection {
	if heartbeatPeriod != 0 {
		go func() {
			for {
				time.Sleep(time.Duration(heartbeatPeriod) * time.Second)
				if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Time{}); err != nil {
					break
				}
			}
		}()
	}

	c := &connection{
		conn:           conn,
		remoteAddr:     remoteAddr,
		reader:         extraReader,
		fragmentConfig: fragmentConfig,
	}
	
	// Initialize connection pool if fragmentation is enabled
	if fragmentConfig != nil && fragmentConfig.Enabled {
		c.connectionPool = make([]*websocket.Conn, 0, 5)
		c.connectionPool = append(c.connectionPool, conn)
	}
	
	return c
}

// Read implements net.Conn.Read()
func (c *connection) Read(b []byte) (int, error) {
	for {
		reader, err := c.getReader()
		if err != nil {
			return 0, err
		}

		nBytes, err := reader.Read(b)
		if errors.Cause(err) == io.EOF {
			c.reader = nil
			continue
		}
		return nBytes, err
	}
}

func (c *connection) getReader() (io.Reader, error) {
	if c.reader != nil {
		return c.reader, nil
	}

	_, reader, err := c.conn.NextReader()
	if err != nil {
		return nil, err
	}
	c.reader = reader
	return reader, nil
}

// Write implements io.Writer.
func (c *connection) Write(b []byte) (int, error) {
	// Check if fragmentation is enabled
	if c.fragmentConfig != nil && c.fragmentConfig.Enabled {
		return c.writeFragmented(b)
	}
	
	if err := c.conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return 0, err
	}
	return len(b), nil
}

// writeFragmented writes data with fragmentation for DPI bypass
func (c *connection) writeFragmented(b []byte) (int, error) {
	totalWritten := 0
	data := b
	
	for len(data) > 0 {
		// Check if we need to switch connection
		currentBytes := c.bytesWritten.Load()
		if currentBytes >= c.fragmentConfig.FragmentSize {
			// Switch to next connection or create new one
			c.switchConnection()
			c.bytesWritten.Store(0)
			
			// Add delay between fragments
			if c.fragmentConfig.FragmentInterval > 0 {
				time.Sleep(c.fragmentConfig.FragmentInterval)
			}
		}
		
		// Calculate chunk size
		remainingInFragment := c.fragmentConfig.FragmentSize - c.bytesWritten.Load()
		chunkSize := len(data)
		if int64(chunkSize) > remainingInFragment {
			chunkSize = int(remainingInFragment)
		}
		
		// Write chunk
		if err := c.conn.WriteMessage(websocket.BinaryMessage, data[:chunkSize]); err != nil {
			return totalWritten, err
		}
		
		c.bytesWritten.Add(int64(chunkSize))
		totalWritten += chunkSize
		data = data[chunkSize:]
	}
	
	return totalWritten, nil
}

// switchConnection switches to next connection in pool
func (c *connection) switchConnection() {
	c.poolMu.RLock()
	poolSize := len(c.connectionPool)
	c.poolMu.RUnlock()
	
	if poolSize == 0 {
		return
	}
	
	// Round-robin through connections
	index := c.currentConnIndex.Add(1) % int32(poolSize)
	
	c.poolMu.RLock()
	if int(index) < len(c.connectionPool) {
		c.conn = c.connectionPool[index]
	}
	c.poolMu.RUnlock()
}

func (c *connection) WriteMultiBuffer(mb buf.MultiBuffer) error {
	mb = buf.Compact(mb)
	mb, err := buf.WriteMultiBuffer(c, mb)
	buf.ReleaseMulti(mb)
	return err
}

func (c *connection) Close() error {
	var errs []interface{}
	
	// Close all connections in the pool if fragmentation is enabled
	if c.fragmentConfig != nil && c.fragmentConfig.Enabled {
		c.poolMu.Lock()
		for _, conn := range c.connectionPool {
			if conn != nil {
				if err := conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second*5)); err != nil {
					errs = append(errs, err)
				}
				if err := conn.Close(); err != nil {
					errs = append(errs, err)
				}
			}
		}
		c.connectionPool = nil
		c.poolMu.Unlock()
	} else {
		// Normal close for non-fragmented connections
		if err := c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second*5)); err != nil {
			errs = append(errs, err)
		}
		if err := c.conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	
	if len(errs) > 0 {
		return errors.New("failed to close connection").Base(errors.New(serial.Concat(errs...)))
	}
	return nil
}

func (c *connection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *connection) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *connection) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}

func (c *connection) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *connection) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
