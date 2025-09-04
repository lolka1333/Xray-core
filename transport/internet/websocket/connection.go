package websocket

import (
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/transport/internet/fragmenter"
)

var _ buf.Writer = (*connection)(nil)

// connection is a wrapper for net.Conn over WebSocket connection.
// remoteAddr is used to pass "virtual" remote IP addresses in X-Forwarded-For.
// so we shouldn't directly read it form conn.
type connection struct {
	conn           *websocket.Conn
	reader         io.Reader
	remoteAddr     net.Addr
	fragmentWriter *fragmenter.FragmentWriter
	fragmentConfig *fragmenter.FragmentConfig
}

func NewConnection(conn *websocket.Conn, remoteAddr net.Addr, extraReader io.Reader, heartbeatPeriod uint32) *connection {
	return NewConnectionWithDPI(conn, remoteAddr, extraReader, heartbeatPeriod, nil)
}

func NewConnectionWithDPI(conn *websocket.Conn, remoteAddr net.Addr, extraReader io.Reader, heartbeatPeriod uint32, dpiConfig *fragmenter.FragmentConfig) *connection {
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
		fragmentConfig: dpiConfig,
	}
	
	// Initialize fragment writer if DPI bypass is enabled
	if dpiConfig != nil && dpiConfig.Enabled {
		c.fragmentWriter = fragmenter.NewFragmentWriter(c, dpiConfig)
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
	// If DPI bypass is enabled and we have a fragment writer, use it
	if c.fragmentWriter != nil && c.fragmentConfig != nil && c.fragmentConfig.Enabled {
		// Fragment the data and send in chunks
		return c.writeFragmented(b)
	}
	
	// Normal write without fragmentation
	if err := c.conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return 0, err
	}
	return len(b), nil
}

// writeFragmented writes data in fragments to bypass DPI
func (c *connection) writeFragmented(b []byte) (int, error) {
	totalWritten := 0
	remaining := b
	
	for len(remaining) > 0 {
		// Calculate fragment size
		fragmentSize := c.fragmentConfig.FragmentSize
		if fragmentSize <= 0 {
			fragmentSize = fragmenter.DefaultFragmentSize
		}
		if int32(len(remaining)) < fragmentSize {
			fragmentSize = int32(len(remaining))
		}
		
		// Write fragment
		if err := c.conn.WriteMessage(websocket.BinaryMessage, remaining[:fragmentSize]); err != nil {
			return totalWritten, err
		}
		
		totalWritten += int(fragmentSize)
		remaining = remaining[fragmentSize:]
		
		// Apply delay between fragments if configured
		if c.fragmentConfig.FragmentDelay > 0 && len(remaining) > 0 {
			time.Sleep(time.Duration(c.fragmentConfig.FragmentDelay) * time.Millisecond)
		}
	}
	
	return totalWritten, nil
}

func (c *connection) WriteMultiBuffer(mb buf.MultiBuffer) error {
	mb = buf.Compact(mb)
	
	// If DPI bypass is enabled, use fragmented writing
	if c.fragmentWriter != nil && c.fragmentConfig != nil && c.fragmentConfig.Enabled {
		err := c.fragmentWriter.WriteMultiBuffer(mb)
		buf.ReleaseMulti(mb)
		return err
	}
	
	// Normal write without fragmentation
	mb, err := buf.WriteMultiBuffer(c, mb)
	buf.ReleaseMulti(mb)
	return err
}

func (c *connection) Close() error {
	var errs []interface{}
	if err := c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second*5)); err != nil {
		errs = append(errs, err)
	}
	if err := c.conn.Close(); err != nil {
		errs = append(errs, err)
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
