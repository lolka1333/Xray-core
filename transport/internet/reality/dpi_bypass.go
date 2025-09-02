package reality

import (
	"crypto/rand"
	"crypto/tls"
	"net"
	"time"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net/cnc"
)

// DPIBypassConfig contains configuration for DPI bypass techniques
type DPIBypassConfig struct {
	// Enable TLS fingerprint randomization
	RandomizeTLSFingerprint bool

	// Enable ESNI/ECH (Encrypted SNI/Client Hello)
	EnableECH bool

	// Enable TLS padding
	EnablePadding bool
	MinPaddingSize int
	MaxPaddingSize int

	// Enable traffic obfuscation
	EnableObfuscation bool

	// Enable TCP timestamp manipulation
	ManipulateTCPTimestamps bool

	// Enable TTL manipulation
	ManipulateTTL bool
	TTLValue      int

	// Enable window size manipulation
	ManipulateWindowSize bool
	WindowSize          int
}

// ApplyDPIBypass applies DPI bypass techniques to the connection
func ApplyDPIBypass(conn net.Conn, config *DPIBypassConfig) (net.Conn, error) {
	if config == nil {
		return conn, nil
	}

	// Apply TCP-level manipulations
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if err := applyTCPManipulations(tcpConn, config); err != nil {
			return nil, err
		}
	}

	// Wrap connection with DPI bypass layer
	return &DPIBypassConn{
		Conn:   conn,
		config: config,
	}, nil
}

// DPIBypassConn wraps a connection with DPI bypass techniques
type DPIBypassConn struct {
	net.Conn
	config *DPIBypassConfig
}

// Write implements net.Conn with DPI bypass
func (c *DPIBypassConn) Write(b []byte) (int, error) {
	// Apply obfuscation if enabled
	if c.config.EnableObfuscation {
		b = obfuscateData(b)
	}

	// Apply padding if enabled
	if c.config.EnablePadding {
		b = addPadding(b, c.config.MinPaddingSize, c.config.MaxPaddingSize)
	}

	return c.Conn.Write(b)
}

// Read implements net.Conn with DPI bypass
func (c *DPIBypassConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if err != nil {
		return n, err
	}

	// Remove obfuscation if enabled
	if c.config.EnableObfuscation && n > 0 {
		deobfuscateData(b[:n])
	}

	// Remove padding if present
	if c.config.EnablePadding && n > 0 {
		n = removePadding(b[:n])
	}

	return n, nil
}

// applyTCPManipulations applies TCP-level manipulations
func applyTCPManipulations(conn *net.TCPConn, config *DPIBypassConfig) error {
	// Set TCP_NODELAY to disable Nagle's algorithm
	if err := conn.SetNoDelay(true); err != nil {
		return err
	}

	// Set keep-alive with aggressive parameters
	if err := conn.SetKeepAlive(true); err != nil {
		return err
	}
	if err := conn.SetKeepAlivePeriod(30 * time.Second); err != nil {
		return err
	}

	// Additional platform-specific manipulations would go here
	// (TTL, window size, timestamps, etc.)

	return nil
}

// obfuscateData applies simple XOR obfuscation
func obfuscateData(data []byte) []byte {
	key := make([]byte, 1)
	rand.Read(key)
	
	result := make([]byte, len(data)+1)
	result[0] = key[0] // Store key in first byte
	
	for i, b := range data {
		result[i+1] = b ^ key[0]
	}
	
	return result
}

// deobfuscateData removes XOR obfuscation
func deobfuscateData(data []byte) {
	if len(data) < 2 {
		return
	}
	
	key := data[0]
	for i := 1; i < len(data); i++ {
		data[i-1] = data[i] ^ key
	}
}

// addPadding adds random padding to data
func addPadding(data []byte, minSize, maxSize int) []byte {
	paddingSize := minSize
	if maxSize > minSize {
		paddingSize = minSize + randInt(maxSize-minSize)
	}
	
	padding := make([]byte, paddingSize)
	rand.Read(padding)
	
	// Format: [original_length:4][data][padding]
	result := make([]byte, 4+len(data)+paddingSize)
	
	// Store original length
	result[0] = byte(len(data) >> 24)
	result[1] = byte(len(data) >> 16)
	result[2] = byte(len(data) >> 8)
	result[3] = byte(len(data))
	
	// Copy data
	copy(result[4:], data)
	
	// Add padding
	copy(result[4+len(data):], padding)
	
	return result
}

// removePadding removes padding from data
func removePadding(data []byte) int {
	if len(data) < 4 {
		return len(data)
	}
	
	// Extract original length
	originalLen := int(data[0])<<24 | int(data[1])<<16 | int(data[2])<<8 | int(data[3])
	
	if originalLen > len(data)-4 {
		return len(data)
	}
	
	// Move data to beginning
	copy(data, data[4:4+originalLen])
	
	return originalLen
}

// randInt returns a random integer
func randInt(max int) int {
	b := make([]byte, 4)
	rand.Read(b)
	return int(b[0])<<24|int(b[1])<<16|int(b[2])<<8|int(b[3]) % max
}

// GenerateRandomTLSConfig generates a TLS config with randomized parameters
func GenerateRandomTLSConfig() *tls.Config {
	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS13,
	}

	// Randomize cipher suites order
	cipherSuites := []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_AES_128_GCM_SHA256,
		tls.TLS_AES_256_GCM_SHA384,
		tls.TLS_CHACHA20_POLY1305_SHA256,
	}

	// Shuffle cipher suites
	for i := range cipherSuites {
		j := randInt(len(cipherSuites))
		cipherSuites[i], cipherSuites[j] = cipherSuites[j], cipherSuites[i]
	}

	config.CipherSuites = cipherSuites

	// Randomize curve preferences
	curves := []tls.CurveID{
		tls.X25519,
		tls.CurveP256,
		tls.CurveP384,
		tls.CurveP521,
	}

	// Shuffle curves
	for i := range curves {
		j := randInt(len(curves))
		curves[i], curves[j] = curves[j], curves[i]
	}

	config.CurvePreferences = curves

	return config
}

// CreateDecoyTraffic creates decoy traffic to confuse DPI
func CreateDecoyTraffic(conn net.Conn, duration time.Duration) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(duration)
	
	for {
		select {
		case <-ticker.C:
			// Send small random packet
			decoy := make([]byte, 10+randInt(50))
			rand.Read(decoy)
			conn.Write(decoy)
		case <-timeout:
			return
		}
	}
}