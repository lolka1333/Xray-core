package splithttp

import (
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xtls/xray-core/common/errors"
)

// FragmentConfig holds fragmentation settings for DPI bypass
type FragmentConfig struct {
	Enabled          bool
	FragmentSize     int64 // Size in bytes
	FragmentInterval time.Duration
}

// FragmentedWriter wraps a writer to fragment data for DPI bypass
type FragmentedWriter struct {
	writer         io.Writer
	config         *FragmentConfig
	bytesWritten   atomic.Int64
	mu             sync.Mutex
	connectionPool []io.Writer
	poolMu         sync.RWMutex
	currentIndex   atomic.Int32
	createWriter   func() (io.Writer, error)
}

// NewFragmentedWriter creates a new fragmented writer
func NewFragmentedWriter(writer io.Writer, config *FragmentConfig) *FragmentedWriter {
	if config == nil || !config.Enabled {
		return &FragmentedWriter{
			writer: writer,
			config: config,
		}
	}

	fw := &FragmentedWriter{
		writer:         writer,
		config:         config,
		connectionPool: make([]io.Writer, 0, 5),
	}

	// Add the initial writer to the pool
	fw.connectionPool = append(fw.connectionPool, writer)

	return fw
}

// SetWriterFactory sets the function to create new writers
func (fw *FragmentedWriter) SetWriterFactory(createWriter func() (io.Writer, error)) {
	fw.createWriter = createWriter
}

// Write implements io.Writer with fragmentation
func (fw *FragmentedWriter) Write(b []byte) (int, error) {
	if fw.config == nil || !fw.config.Enabled {
		// Passthrough if fragmentation is disabled
		return fw.writer.Write(b)
	}

	fw.mu.Lock()
	defer fw.mu.Unlock()

	totalWritten := 0
	data := b

	for len(data) > 0 {
		// Check if we need to switch writer
		currentBytes := fw.bytesWritten.Load()
		if currentBytes >= fw.config.FragmentSize {
			// Get or create a new writer
			if err := fw.switchWriter(); err != nil {
				return totalWritten, err
			}
			fw.bytesWritten.Store(0)

			// Add delay between fragments
			if fw.config.FragmentInterval > 0 {
				time.Sleep(fw.config.FragmentInterval)
			}
		}

		// Calculate chunk size
		remainingInFragment := fw.config.FragmentSize - fw.bytesWritten.Load()
		chunkSize := len(data)
		if int64(chunkSize) > remainingInFragment {
			chunkSize = int(remainingInFragment)
		}

		// Write chunk
		n, err := fw.writer.Write(data[:chunkSize])
		if err != nil {
			return totalWritten, err
		}

		fw.bytesWritten.Add(int64(n))
		totalWritten += n
		data = data[n:]
	}

	return totalWritten, nil
}

// switchWriter switches to the next writer in the pool or creates a new one
func (fw *FragmentedWriter) switchWriter() error {
	fw.poolMu.RLock()
	poolSize := len(fw.connectionPool)
	fw.poolMu.RUnlock()

	if poolSize == 0 {
		return errors.New("no writers in pool")
	}

	// Try to get next writer from pool
	index := fw.currentIndex.Add(1) % int32(poolSize)

	fw.poolMu.RLock()
	if int(index) < len(fw.connectionPool) {
		fw.writer = fw.connectionPool[index]
	}
	fw.poolMu.RUnlock()

	// Create more writers if needed and possible
	if poolSize < 5 && fw.createWriter != nil {
		go func() {
			newWriter, err := fw.createWriter()
			if err == nil {
				fw.poolMu.Lock()
				if len(fw.connectionPool) < 5 {
					fw.connectionPool = append(fw.connectionPool, newWriter)
				}
				fw.poolMu.Unlock()
			}
		}()
	}

	return nil
}

// Close closes all writers in the pool
func (fw *FragmentedWriter) Close() error {
	fw.poolMu.Lock()
	defer fw.poolMu.Unlock()

	var errs []error
	for _, writer := range fw.connectionPool {
		if closer, ok := writer.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	fw.connectionPool = nil

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// GetFragmentConfig creates a FragmentConfig from the Config
func GetFragmentConfig(config *Config) *FragmentConfig {
	if config == nil || !config.EnableFragmentation {
		return nil
	}

	fragmentSize := int64(config.GetFragmentSize()) * 1024 // Convert KB to bytes
	if fragmentSize == 0 {
		fragmentSize = 15 * 1024 // Default 15KB
	}

	fragmentInterval := time.Duration(config.GetFragmentInterval()) * time.Millisecond
	if fragmentInterval == 0 {
		fragmentInterval = 10 * time.Millisecond // Default 10ms
	}

	return &FragmentConfig{
		Enabled:          true,
		FragmentSize:     fragmentSize,
		FragmentInterval: fragmentInterval,
	}
}