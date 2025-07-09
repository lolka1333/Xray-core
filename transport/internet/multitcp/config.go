package multitcp

import (
	"errors"
	"time"

	"github.com/xtls/xray-core/transport/internet"
)

// GetMaxDataPerConn returns the maximum data per connection
func (c *Config) GetMaxDataPerConn() int64 {
	if c.MaxDataPerConn <= 0 {
		return DefaultMaxDataPerConn
	}
	return int64(c.MaxDataPerConn)
}

// GetMaxConnections returns the maximum number of connections
func (c *Config) GetMaxConnections() int {
	if c.MaxConnections <= 0 {
		return DefaultMaxConnections
	}
	return int(c.MaxConnections)
}

// GetConnTimeout returns the connection timeout
func (c *Config) GetConnTimeout() time.Duration {
	if c.ConnTimeout <= 0 {
		return DefaultConnTimeout
	}
	return time.Duration(c.ConnTimeout) * time.Second
}

// GetCleanupInterval returns the cleanup interval
func (c *Config) GetCleanupInterval() time.Duration {
	if c.CleanupInterval <= 0 {
		return 60 * time.Second
	}
	return time.Duration(c.CleanupInterval) * time.Second
}

// GetMinDataSize returns the minimum data size for adaptive algorithm
func (c *Config) GetMinDataSize() int64 {
	if c.MinDataSize <= 0 {
		return 1024 // 1KB minimum
	}
	return int64(c.MinDataSize)
}

// GetMaxDataSize returns the maximum data size for adaptive algorithm
func (c *Config) GetMaxDataSize() int64 {
	if c.MaxDataSize <= 0 {
		return 20 * 1024 // 20KB maximum
	}
	return int64(c.MaxDataSize)
}

// IsPoolingEnabled returns whether connection pooling is enabled
func (c *Config) IsPoolingEnabled() bool {
	return c.EnablePooling
}

// IsAdaptiveSizeEnabled returns whether adaptive sizing is enabled
func (c *Config) IsAdaptiveSizeEnabled() bool {
	return c.AdaptiveSize
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.GetMaxDataPerConn() < 1024 {
		return errors.New("max_data_per_conn must be at least 1024 bytes")
	}
	if c.GetMaxConnections() < 1 {
		return errors.New("max_connections must be at least 1")
	}
	if c.GetConnTimeout() < time.Second {
		return errors.New("conn_timeout must be at least 1 second")
	}
	if c.IsAdaptiveSizeEnabled() && c.GetMinDataSize() >= c.GetMaxDataSize() {
		return errors.New("min_data_size must be less than max_data_size")
	}
	return nil
}

// GetNormalizedConfig returns a normalized configuration with defaults
func GetNormalizedConfig(config *Config) *Config {
	if config == nil {
		return &Config{
			MaxDataPerConn:  DefaultMaxDataPerConn,
			MaxConnections:  DefaultMaxConnections,
			ConnTimeout:     uint32(DefaultConnTimeout.Seconds()),
			EnablePooling:   true,
			CleanupInterval: 60,
			AdaptiveSize:    false,
			MinDataSize:     1024,
			MaxDataSize:     20 * 1024,
		}
	}
	return config
}

// ConfigFromStreamSettings extracts MultiTCP config from stream settings
func ConfigFromStreamSettings(settings *internet.MemoryStreamConfig) *Config {
	if settings == nil {
		return nil
	}
	
	if config, ok := settings.ProtocolSettings.(*Config); ok {
		return config
	}
	
	return nil
}