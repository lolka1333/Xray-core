package websocket

import (
	"net/http"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/transport/internet"
)

func (c *Config) GetNormalizedPath() string {
	path := c.Path
	if path == "" {
		return "/"
	}
	if path[0] != '/' {
		return "/" + path
	}
	return path
}

func (c *Config) GetRequestHeader() http.Header {
	header := http.Header{}
	for k, v := range c.Header {
		header.Add(k, v)
	}
	return header
}

// GetFragmentSize returns the fragment size for DPI bypass
func (c *Config) GetFragmentSize() uint32 {
	if c.FragmentSize > 0 {
		return c.FragmentSize
	}
	return 15 // Default 15KB
}

// GetFragmentInterval returns the fragment interval in milliseconds
func (c *Config) GetFragmentInterval() uint32 {
	if c.FragmentInterval > 0 {
		return c.FragmentInterval
	}
	return 10 // Default 10ms
}

func init() {
	common.Must(internet.RegisterProtocolConfigCreator(protocolName, func() interface{} {
		return new(Config)
	}))
}
