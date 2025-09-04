package splithttp

import (
	"io"
	"net"
	"time"
	
	"github.com/xtls/xray-core/transport/internet/fragmenter"
)

type splitConn struct {
	writer         io.WriteCloser
	reader         io.ReadCloser
	remoteAddr     net.Addr
	localAddr      net.Addr
	onClose        func()
	fragmentWriter *fragmenter.FragmentWriter
	fragmentConfig *fragmenter.FragmentConfig
}

func (c *splitConn) Write(b []byte) (int, error) {
	// If DPI bypass is enabled and we have a fragment writer, use it
	if c.fragmentWriter != nil && c.fragmentConfig != nil && c.fragmentConfig.Enabled {
		return c.fragmentWriter.Write(b)
	}
	
	// Normal write without fragmentation
	return c.writer.Write(b)
}

func (c *splitConn) Read(b []byte) (int, error) {
	return c.reader.Read(b)
}

func (c *splitConn) Close() error {
	if c.onClose != nil {
		c.onClose()
	}

	err := c.writer.Close()
	err2 := c.reader.Close()
	if err != nil {
		return err
	}

	if err2 != nil {
		return err
	}

	return nil
}

func (c *splitConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *splitConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *splitConn) SetDeadline(t time.Time) error {
	// TODO cannot do anything useful
	return nil
}

func (c *splitConn) SetReadDeadline(t time.Time) error {
	// TODO cannot do anything useful
	return nil
}

func (c *splitConn) SetWriteDeadline(t time.Time) error {
	// TODO cannot do anything useful
	return nil
}
