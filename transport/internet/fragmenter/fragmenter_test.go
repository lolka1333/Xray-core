package fragmenter

import (
	"bytes"
	"io"
	"testing"
)

func TestFragmentWriter(t *testing.T) {
	tests := []struct {
		name         string
		config       *FragmentConfig
		inputSize    int
		expectedWrites int
	}{
		{
			name: "Disabled fragmentation",
			config: &FragmentConfig{
				Enabled: false,
			},
			inputSize:    30000,
			expectedWrites: 1,
		},
		{
			name: "15KB fragments",
			config: &FragmentConfig{
				Enabled:      true,
				FragmentSize: 15360,
			},
			inputSize:    30000,
			expectedWrites: 2,
		},
		{
			name: "10KB fragments",
			config: &FragmentConfig{
				Enabled:      true,
				FragmentSize: 10240,
			},
			inputSize:    30000,
			expectedWrites: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test data
			testData := make([]byte, tt.inputSize)
			for i := range testData {
				testData[i] = byte(i % 256)
			}

			// Create a buffer to write to
			var buf bytes.Buffer
			writeCount := 0
			
			// Create a custom writer that counts writes
			countingWriter := &countingWriter{
				Writer: &buf,
				count:  &writeCount,
			}

			// Create fragment writer
			fw := NewFragmentWriter(countingWriter, tt.config)

			// Write data
			n, err := fw.Write(testData)
			if err != nil {
				t.Fatalf("Write failed: %v", err)
			}
			if n != tt.inputSize {
				t.Fatalf("Expected to write %d bytes, wrote %d", tt.inputSize, n)
			}

			// Verify data integrity
			if !bytes.Equal(buf.Bytes(), testData) {
				t.Fatal("Data corruption detected")
			}

			// Verify number of writes (only for enabled fragmentation)
			if tt.config.Enabled && writeCount != tt.expectedWrites {
				t.Fatalf("Expected %d writes, got %d", tt.expectedWrites, writeCount)
			}
		})
	}
}

func TestFragmentSizeValidation(t *testing.T) {
	tests := []struct {
		name           string
		inputSize      int32
		expectedSize   int32
	}{
		{
			name:          "Zero size defaults to 15KB",
			inputSize:     0,
			expectedSize:  DefaultFragmentSize,
		},
		{
			name:          "Too large size capped at 20KB",
			inputSize:     30000,
			expectedSize:  MaxFragmentSize,
		},
		{
			name:          "Too small size set to 10KB",
			inputSize:     5000,
			expectedSize:  MinFragmentSize,
		},
		{
			name:          "Valid size unchanged",
			inputSize:     12288,
			expectedSize:  12288,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &FragmentConfig{
				Enabled:      true,
				FragmentSize: tt.inputSize,
			}
			
			fw := NewFragmentWriter(io.Discard, config)
			
			if fw.config.FragmentSize != tt.expectedSize {
				t.Fatalf("Expected fragment size %d, got %d", tt.expectedSize, fw.config.FragmentSize)
			}
		})
	}
}

func TestRandomFragmentSize(t *testing.T) {
	config := &FragmentConfig{
		Enabled:    true,
		RandomSize: true,
		MinSize:    10240,
		MaxSize:    20480,
	}

	fw := NewFragmentWriter(io.Discard, config)

	// Test multiple times to ensure randomness
	sizes := make(map[int32]bool)
	for i := 0; i < 100; i++ {
		size := fw.getFragmentSize()
		if size < config.MinSize || size > config.MaxSize {
			t.Fatalf("Fragment size %d out of range [%d, %d]", size, config.MinSize, config.MaxSize)
		}
		sizes[size] = true
	}

	// Should have at least some variation
	if len(sizes) < 2 {
		t.Fatal("Random size generation not working")
	}
}

// Helper type for counting writes
type countingWriter struct {
	io.Writer
	count *int
}

func (w *countingWriter) Write(p []byte) (int, error) {
	*w.count++
	return w.Writer.Write(p)
}