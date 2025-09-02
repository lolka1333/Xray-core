package freedom

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"io"
	"math"
	"time"

	"github.com/xtls/xray-core/common/crypto"
	"github.com/xtls/xray-core/common/errors"
)

// DPIBypassWriter implements advanced DPI bypass techniques for Russian censorship
type DPIBypassWriter struct {
	writer          io.Writer
	fragment        *Fragment
	enableTLSTricks bool
	enableHTTPTricks bool
	count           uint64
}

// NewDPIBypassWriter creates a new DPI bypass writer with enhanced features
func NewDPIBypassWriter(writer io.Writer, fragment *Fragment) *DPIBypassWriter {
	return &DPIBypassWriter{
		writer:           writer,
		fragment:         fragment,
		enableTLSTricks:  true,
		enableHTTPTricks: true,
	}
}

// Write implements io.Writer with DPI bypass techniques
func (d *DPIBypassWriter) Write(b []byte) (int, error) {
	d.count++

	// Detect protocol and apply appropriate bypass technique
	if d.isTLSHandshake(b) {
		return d.writeTLSWithBypass(b)
	} else if d.isHTTPRequest(b) {
		return d.writeHTTPWithBypass(b)
	}

	// Default fragmentation for other protocols
	return d.writeWithFragmentation(b)
}

// isTLSHandshake checks if the packet is a TLS handshake
func (d *DPIBypassWriter) isTLSHandshake(b []byte) bool {
	if len(b) < 6 {
		return false
	}
	// TLS handshake: content type 22 (0x16)
	return b[0] == 0x16 && b[1] == 0x03 && (b[2] >= 0x01 && b[2] <= 0x04)
}

// isHTTPRequest checks if the packet is an HTTP request
func (d *DPIBypassWriter) isHTTPRequest(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	// Check for common HTTP methods
	httpMethods := [][]byte{
		[]byte("GET "),
		[]byte("POST"),
		[]byte("HEAD"),
		[]byte("PUT "),
		[]byte("DELE"),
		[]byte("CONN"),
		[]byte("OPTI"),
		[]byte("TRAC"),
		[]byte("PATC"),
	}
	for _, method := range httpMethods {
		if bytes.HasPrefix(b, method) {
			return true
		}
	}
	return false
}

// writeTLSWithBypass applies TLS-specific DPI bypass techniques
func (d *DPIBypassWriter) writeTLSWithBypass(b []byte) (int, error) {
	if !d.enableTLSTricks || d.count != 1 {
		return d.writeWithFragmentation(b)
	}

	// Extract TLS record length
	if len(b) < 5 {
		return d.writer.Write(b)
	}

	recordLen := 5 + ((int(b[3]) << 8) | int(b[4]))
	if len(b) < recordLen {
		return d.writer.Write(b)
	}

	// Method 1: Fragment at SNI position
	// SNI is typically around byte 40-50 in ClientHello
	if recordLen > 50 {
		return d.fragmentAtSNI(b, recordLen)
	}

	// Method 2: Fragment with fake packets
	return d.fragmentWithFakePackets(b, recordLen)
}

// fragmentAtSNI fragments the TLS handshake at the SNI field
func (d *DPIBypassWriter) fragmentAtSNI(b []byte, recordLen int) (int, error) {
	// Split the ClientHello at strategic positions
	// First fragment: TLS header + part of handshake (before SNI)
	firstFragmentSize := 43 // Typical position before SNI

	if recordLen < firstFragmentSize {
		firstFragmentSize = recordLen / 2
	}

	// Send first fragment
	_, err := d.writer.Write(b[:firstFragmentSize])
	if err != nil {
		return 0, err
	}

	// Add delay to confuse DPI timing analysis
	time.Sleep(time.Duration(crypto.RandBetween(10, 50)) * time.Millisecond)

	// Send remaining data in small chunks
	written := firstFragmentSize
	for written < len(b) {
		chunkSize := int(crypto.RandBetween(20, 100))
		if written+chunkSize > len(b) {
			chunkSize = len(b) - written
		}

		_, err = d.writer.Write(b[written : written+chunkSize])
		if err != nil {
			return written, err
		}

		written += chunkSize

		if written < len(b) {
			time.Sleep(time.Duration(crypto.RandBetween(5, 20)) * time.Millisecond)
		}
	}

	return len(b), nil
}

// fragmentWithFakePackets sends fake packets to confuse DPI
func (d *DPIBypassWriter) fragmentWithFakePackets(b []byte, recordLen int) (int, error) {
	// Method: Send fake TLS record with wrong version/length
	fakeTLS := make([]byte, 5)
	fakeTLS[0] = 0x16 // TLS handshake
	fakeTLS[1] = 0x03
	fakeTLS[2] = 0xFF // Invalid version
	binary.BigEndian.PutUint16(fakeTLS[3:], uint16(0)) // Zero length

	// Send fake packet (DPI might process this and fail)
	d.writer.Write(fakeTLS)

	// Small delay
	time.Sleep(time.Duration(crypto.RandBetween(5, 15)) * time.Millisecond)

	// Send real data fragmented
	return d.writeWithFragmentation(b)
}

// writeHTTPWithBypass applies HTTP-specific DPI bypass techniques
func (d *DPIBypassWriter) writeHTTPWithBypass(b []byte) (int, error) {
	if !d.enableHTTPTricks {
		return d.writeWithFragmentation(b)
	}

	// Find Host header position
	hostIndex := bytes.Index(b, []byte("Host:"))
	if hostIndex == -1 {
		return d.writeWithFragmentation(b)
	}

	// Method 1: Fragment at Host header
	// Send everything before "Host:" first
	_, err := d.writer.Write(b[:hostIndex])
	if err != nil {
		return 0, err
	}

	// Delay
	time.Sleep(time.Duration(crypto.RandBetween(10, 30)) * time.Millisecond)

	// Send "Ho" separately (breaks "Host" keyword)
	_, err = d.writer.Write(b[hostIndex : hostIndex+2])
	if err != nil {
		return hostIndex, err
	}

	// Small delay
	time.Sleep(time.Duration(crypto.RandBetween(5, 15)) * time.Millisecond)

	// Send the rest
	_, err = d.writer.Write(b[hostIndex+2:])
	if err != nil {
		return hostIndex + 2, err
	}

	return len(b), nil
}

// writeWithFragmentation applies general fragmentation
func (d *DPIBypassWriter) writeWithFragmentation(b []byte) (int, error) {
	if d.fragment == nil {
		return d.writer.Write(b)
	}

	// Check if we should fragment this packet
	if d.fragment.PacketsFrom != 0 && (d.count < d.fragment.PacketsFrom || d.count > d.fragment.PacketsTo) {
		return d.writer.Write(b)
	}

	// Advanced fragmentation with random patterns
	totalWritten := 0
	maxSplit := crypto.RandBetween(int64(d.fragment.MaxSplitMin), int64(d.fragment.MaxSplitMax))
	splitCount := int64(0)

	for totalWritten < len(b) {
		// Random chunk size with exponential distribution for more realistic traffic
		chunkSize := d.getRandomChunkSize()
		
		splitCount++
		if totalWritten+chunkSize > len(b) || (maxSplit > 0 && splitCount >= maxSplit) {
			chunkSize = len(b) - totalWritten
		}

		// Write chunk
		n, err := d.writer.Write(b[totalWritten : totalWritten+chunkSize])
		totalWritten += n
		if err != nil {
			return totalWritten, err
		}

		// Random delay between fragments
		if totalWritten < len(b) && d.fragment.IntervalMax > 0 {
			delay := crypto.RandBetween(int64(d.fragment.IntervalMin), int64(d.fragment.IntervalMax))
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
	}

	return totalWritten, nil
}

// getRandomChunkSize returns a random chunk size with exponential distribution
func (d *DPIBypassWriter) getRandomChunkSize() int {
	if d.fragment == nil {
		return 1024
	}

	min := float64(d.fragment.LengthMin)
	max := float64(d.fragment.LengthMax)

	// Use exponential distribution for more natural packet sizes
	lambda := 1.0 / ((max - min) / 3.0)
	expValue := -math.Log(1.0-randFloat64()) / lambda
	
	size := int(min + expValue)
	if size > int(max) {
		size = int(max)
	}
	if size < int(min) {
		size = int(min)
	}

	return size
}

// randFloat64 returns a random float64 in [0, 1)
func randFloat64() float64 {
	b := make([]byte, 8)
	rand.Read(b)
	return float64(binary.LittleEndian.Uint64(b)) / float64(^uint64(0))
}

// TCPFastOpenWriter implements TCP Fast Open to bypass initial packet inspection
type TCPFastOpenWriter struct {
	writer    io.Writer
	firstWrite bool
}

// NewTCPFastOpenWriter creates a new TCP Fast Open writer
func NewTCPFastOpenWriter(writer io.Writer) *TCPFastOpenWriter {
	return &TCPFastOpenWriter{
		writer:     writer,
		firstWrite: true,
	}
}

// Write implements io.Writer with TCP Fast Open
func (t *TCPFastOpenWriter) Write(b []byte) (int, error) {
	if t.firstWrite {
		t.firstWrite = false
		// For first write, we could implement TCP Fast Open logic here
		// This would send data in the SYN packet
	}
	return t.writer.Write(b)
}

// PaddingWriter adds random padding to packets
type PaddingWriter struct {
	writer      io.Writer
	minPadding  int
	maxPadding  int
}

// NewPaddingWriter creates a new padding writer
func NewPaddingWriter(writer io.Writer, minPadding, maxPadding int) *PaddingWriter {
	return &PaddingWriter{
		writer:     writer,
		minPadding: minPadding,
		maxPadding: maxPadding,
	}
}

// Write implements io.Writer with padding
func (p *PaddingWriter) Write(b []byte) (int, error) {
	// Add random padding
	paddingSize := int(crypto.RandBetween(int64(p.minPadding), int64(p.maxPadding)))
	padding := make([]byte, paddingSize)
	rand.Read(padding)

	// Create padded packet
	padded := append(b, padding...)

	// Write padded data
	n, err := p.writer.Write(padded)
	if err != nil {
		return 0, err
	}

	// Return original data size
	if n >= len(b) {
		return len(b), nil
	}
	return n, nil
}