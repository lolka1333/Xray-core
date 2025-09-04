package fragmenter

import (
	"io"
	"sync"
	"time"

	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
)

const (
	// Default fragment size to avoid DPI detection (15KB)
	DefaultFragmentSize = 15 * 1024
	// Maximum fragment size (20KB)
	MaxFragmentSize = 20 * 1024
	// Minimum fragment size (10KB)
	MinFragmentSize = 10 * 1024
)

// FragmentConfig holds configuration for DPI bypass fragmentation
type FragmentConfig struct {
	// Enable DPI bypass fragmentation
	Enabled bool
	// Fragment size in bytes (default: 15KB)
	FragmentSize int32
	// Delay between fragments in milliseconds
	FragmentDelay int32
	// Use random fragment sizes between min and max
	RandomSize bool
	// Minimum random size (if RandomSize is true)
	MinSize int32
	// Maximum random size (if RandomSize is true)
	MaxSize int32
}

// FragmentWriter wraps an io.Writer and fragments data to bypass DPI
type FragmentWriter struct {
	writer       io.Writer
	config       *FragmentConfig
	bytesWritten int64
	mu           sync.Mutex
	lastWrite    time.Time
}

// NewFragmentWriter creates a new FragmentWriter
func NewFragmentWriter(writer io.Writer, config *FragmentConfig) *FragmentWriter {
	if config == nil {
		config = &FragmentConfig{
			Enabled:      false,
			FragmentSize: DefaultFragmentSize,
		}
	}

	// Validate and adjust fragment size
	if config.FragmentSize <= 0 {
		config.FragmentSize = DefaultFragmentSize
	} else if config.FragmentSize > MaxFragmentSize {
		config.FragmentSize = MaxFragmentSize
	} else if config.FragmentSize < MinFragmentSize {
		config.FragmentSize = MinFragmentSize
	}

	// Validate random size configuration
	if config.RandomSize {
		if config.MinSize <= 0 {
			config.MinSize = MinFragmentSize
		}
		if config.MaxSize <= 0 || config.MaxSize > MaxFragmentSize {
			config.MaxSize = MaxFragmentSize
		}
		if config.MinSize > config.MaxSize {
			config.MinSize = MinFragmentSize
			config.MaxSize = MaxFragmentSize
		}
	}

	return &FragmentWriter{
		writer:    writer,
		config:    config,
		lastWrite: time.Now(),
	}
}

// Write implements io.Writer with fragmentation
func (fw *FragmentWriter) Write(data []byte) (int, error) {
	if !fw.config.Enabled || len(data) == 0 {
		return fw.writer.Write(data)
	}

	fw.mu.Lock()
	defer fw.mu.Unlock()

	totalWritten := 0
	remaining := data

	for len(remaining) > 0 {
		fragmentSize := fw.getFragmentSize()
		if fragmentSize > int32(len(remaining)) {
			fragmentSize = int32(len(remaining))
		}

		// Apply delay between fragments if configured
		if fw.config.FragmentDelay > 0 && fw.bytesWritten > 0 {
			elapsed := time.Since(fw.lastWrite)
			delay := time.Duration(fw.config.FragmentDelay) * time.Millisecond
			if elapsed < delay {
				time.Sleep(delay - elapsed)
			}
		}

		// Write fragment
		n, err := fw.writer.Write(remaining[:fragmentSize])
		if err != nil {
			return totalWritten, err
		}

		totalWritten += n
		fw.bytesWritten += int64(n)
		fw.lastWrite = time.Now()
		remaining = remaining[n:]

		// If we didn't write the full fragment, stop here
		if n < int(fragmentSize) {
			break
		}
	}

	return totalWritten, nil
}

// WriteMultiBuffer writes a MultiBuffer with fragmentation
func (fw *FragmentWriter) WriteMultiBuffer(mb buf.MultiBuffer) error {
	if !fw.config.Enabled {
		mb, err := buf.WriteMultiBuffer(fw.writer, mb)
		buf.ReleaseMulti(mb)
		return err
	}

	fw.mu.Lock()
	defer fw.mu.Unlock()

	for _, buffer := range mb {
		data := buffer.Bytes()
		remaining := data

		for len(remaining) > 0 {
			fragmentSize := fw.getFragmentSize()
			if fragmentSize > int32(len(remaining)) {
				fragmentSize = int32(len(remaining))
			}

			// Apply delay between fragments if configured
			if fw.config.FragmentDelay > 0 && fw.bytesWritten > 0 {
				elapsed := time.Since(fw.lastWrite)
				delay := time.Duration(fw.config.FragmentDelay) * time.Millisecond
				if elapsed < delay {
					time.Sleep(delay - elapsed)
				}
			}

			// Write fragment
			n, err := fw.writer.Write(remaining[:fragmentSize])
			if err != nil {
				return err
			}

			fw.bytesWritten += int64(n)
			fw.lastWrite = time.Now()
			remaining = remaining[n:]

			// If we didn't write the full fragment, stop here
			if n < int(fragmentSize) {
				return errors.New("incomplete fragment write")
			}
		}
	}

	return nil
}

// getFragmentSize returns the fragment size to use for the next write
func (fw *FragmentWriter) getFragmentSize() int32 {
	if !fw.config.RandomSize {
		return fw.config.FragmentSize
	}

	// Use a simple pseudo-random size between min and max
	// This helps avoid pattern detection
	range_ := fw.config.MaxSize - fw.config.MinSize
	if range_ <= 0 {
		return fw.config.MinSize
	}

	// Simple time-based pseudo-random
	random := time.Now().UnixNano() % int64(range_)
	return fw.config.MinSize + int32(random)
}

// FragmentReader wraps an io.Reader and handles fragmented data
type FragmentReader struct {
	reader io.Reader
	config *FragmentConfig
	buffer []byte
	mu     sync.Mutex
}

// NewFragmentReader creates a new FragmentReader
func NewFragmentReader(reader io.Reader, config *FragmentConfig) *FragmentReader {
	if config == nil {
		config = &FragmentConfig{
			Enabled:      false,
			FragmentSize: DefaultFragmentSize,
		}
	}

	return &FragmentReader{
		reader: reader,
		config: config,
		buffer: make([]byte, 0, MaxFragmentSize),
	}
}

// Read implements io.Reader with fragment handling
func (fr *FragmentReader) Read(p []byte) (int, error) {
	if !fr.config.Enabled {
		return fr.reader.Read(p)
	}

	fr.mu.Lock()
	defer fr.mu.Unlock()

	// If we have buffered data, return it first
	if len(fr.buffer) > 0 {
		n := copy(p, fr.buffer)
		fr.buffer = fr.buffer[n:]
		return n, nil
	}

	// Read new data
	return fr.reader.Read(p)
}

// ConnectionFragmenter manages fragmentation across multiple connections
type ConnectionFragmenter struct {
	config      *FragmentConfig
	connections []io.ReadWriteCloser
	currentIdx  int
	mu          sync.Mutex
}

// NewConnectionFragmenter creates a new ConnectionFragmenter
func NewConnectionFragmenter(config *FragmentConfig) *ConnectionFragmenter {
	if config == nil {
		config = &FragmentConfig{
			Enabled:      false,
			FragmentSize: DefaultFragmentSize,
		}
	}

	return &ConnectionFragmenter{
		config:      config,
		connections: make([]io.ReadWriteCloser, 0),
		currentIdx:  0,
	}
}

// AddConnection adds a new connection to the fragmenter
func (cf *ConnectionFragmenter) AddConnection(conn io.ReadWriteCloser) {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	cf.connections = append(cf.connections, conn)
}

// GetNextConnection returns the next connection for writing a fragment
func (cf *ConnectionFragmenter) GetNextConnection() io.ReadWriteCloser {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	if len(cf.connections) == 0 {
		return nil
	}

	conn := cf.connections[cf.currentIdx]
	cf.currentIdx = (cf.currentIdx + 1) % len(cf.connections)
	return conn
}

// Close closes all connections
func (cf *ConnectionFragmenter) Close() error {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	var firstErr error
	for _, conn := range cf.connections {
		if err := conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	cf.connections = nil
	return firstErr
}