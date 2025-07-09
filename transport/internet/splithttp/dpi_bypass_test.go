package splithttp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResponseFragmenter(t *testing.T) {
	config := &ResponseFragmentationConfig{
		Enabled:      true,
		MaxChunkSize: 1000,
		RandomDelay:  &RangeConfig{From: 0, To: 0}, // No delay for testing
	}

	fragmenter := NewResponseFragmenter(config)

	// Test data larger than chunk size
	testData := make([]byte, 2500)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	recorder := httptest.NewRecorder()
	ctx := context.Background()

	err := fragmenter.FragmentResponse(ctx, testData, recorder)
	if err != nil {
		t.Fatalf("FragmentResponse failed: %v", err)
	}

	if recorder.Body.Len() != len(testData) {
		t.Errorf("Expected %d bytes, got %d", len(testData), recorder.Body.Len())
	}
}

func TestTrafficMasker(t *testing.T) {
	config := &TrafficMaskingConfig{
		UserAgentRotation:         true,
		AcceptHeaders:            []string{"text/html", "application/json"},
		RefererGeneration:        true,
		CookieManagement:         true,
		CompressionSupport:       []string{"gzip", "deflate"},
		BrowserBehaviorSimulation: true,
	}

	masker := NewTrafficMasker(config)

	req := httptest.NewRequest("GET", "https://example.com/test", nil)
	
	masker.MaskRequest(req)

	// Check if headers were added
	if req.Header.Get("User-Agent") == "" {
		t.Error("User-Agent header should be set")
	}

	if req.Header.Get("Accept-Language") == "" {
		t.Error("Accept-Language header should be set")
	}

	if req.Header.Get("Accept-Encoding") == "" {
		t.Error("Accept-Encoding header should be set")
	}

	// Check if cookies were added
	cookies := req.Cookies()
	if len(cookies) == 0 {
		t.Error("Cookies should be added")
	}
}

func TestEntropyReducer(t *testing.T) {
	config := &EntropyReductionConfig{
		Enabled:          true,
		Method:           "structured-padding",
		TargetEntropy:    0.7,
		PatternInjection: true,
	}

	reducer := NewEntropyReducer(config)

	// High entropy data (random bytes)
	highEntropyData := make([]byte, 1000)
	for i := range highEntropyData {
		highEntropyData[i] = byte(i % 256)
	}

	reducedData := reducer.ReduceEntropy(highEntropyData)

	// Check if data was modified (should be larger due to padding)
	if len(reducedData) <= len(highEntropyData) {
		t.Error("Data should be larger after entropy reduction")
	}
}

func TestConnectionManager(t *testing.T) {
	config := &ConnectionManagementConfig{
		MaxConnectionLifetime: 1, // 1 second for testing
		ConnectionRotation:    true,
		ParallelConnections:   2,
		LoadBalancing:        "round-robin",
	}

	manager := NewConnectionManager(config)

	// Mock connection creator
	connCount := 0
	createConn := func() (io.ReadWriteCloser, error) {
		connCount++
		return &mockConn{id: connCount}, nil
	}

	ctx := context.Background()

	// Get first connection
	conn1, err := manager.GetConnection(ctx, createConn)
	if err != nil {
		t.Fatalf("GetConnection failed: %v", err)
	}

	// Get second connection (should be different due to round-robin)
	conn2, err := manager.GetConnection(ctx, createConn)
	if err != nil {
		t.Fatalf("GetConnection failed: %v", err)
	}

	// Check if connections are different
	if conn1.(*mockConn).id == conn2.(*mockConn).id {
		t.Error("Connections should be different with round-robin")
	}

	// Wait for connection lifetime to expire
	time.Sleep(1100 * time.Millisecond)

	// Get connection again - should create new one
	conn3, err := manager.GetConnection(ctx, createConn)
	if err != nil {
		t.Fatalf("GetConnection failed: %v", err)
	}

	if conn3 == conn1 {
		t.Error("Connection should be recreated after lifetime expiry")
	}
}

func TestCDNHopper(t *testing.T) {
	config := &CDNHoppingConfig{
		Enabled:           true,
		Providers:         []string{"cloudflare", "gcore", "aws"},
		RotationInterval:  1, // 1 second for testing
		FailoverThreshold: 2,
	}

	hopper := NewCDNHopper(config)

	// Test initial endpoint
	endpoint1 := hopper.GetCurrentEndpoint("example.com")
	if endpoint1 != "example.com" {
		t.Errorf("Expected example.com, got %s", endpoint1)
	}

	// Test rotation
	if !hopper.ShouldRotate() {
		// Force rotation by triggering failover
		hopper.HandleFailover("cloudflare")
		hopper.HandleFailover("cloudflare")
	}

	// Wait for rotation interval
	time.Sleep(1100 * time.Millisecond)

	if !hopper.ShouldRotate() {
		t.Error("Should rotate after interval")
	}
}

func TestDPIBypassManager(t *testing.T) {
	config := &DPIBypassConfig{
		Enabled: true,
		ResponseFragmentation: &ResponseFragmentationConfig{
			Enabled:      true,
			MaxChunkSize: 1000,
		},
		TrafficMasking: &TrafficMaskingConfig{
			UserAgentRotation: true,
		},
		EntropyReduction: &EntropyReductionConfig{
			Enabled: true,
			Method:  "structured-padding",
		},
	}

	manager := NewDPIBypassManager(config)

	if !manager.IsEnabled() {
		t.Error("Manager should be enabled")
	}

	// Test request processing
	req := httptest.NewRequest("GET", "https://example.com/test", nil)
	manager.ProcessRequest(req)

	if req.Header.Get("User-Agent") == "" {
		t.Error("User-Agent should be set by traffic masking")
	}

	// Test response processing
	testData := make([]byte, 2000)
	recorder := httptest.NewRecorder()
	ctx := context.Background()

	err := manager.ProcessResponseData(ctx, testData, recorder)
	if err != nil {
		t.Fatalf("ProcessResponseData failed: %v", err)
	}

	if recorder.Body.Len() == 0 {
		t.Error("Response should not be empty")
	}
}

// Mock connection for testing
type mockConn struct {
	id int
}

func (m *mockConn) Read(p []byte) (int, error) {
	return 0, nil
}

func (m *mockConn) Write(p []byte) (int, error) {
	return len(p), nil
}

func (m *mockConn) Close() error {
	return nil
}

func TestDPIBypassConfigHelpers(t *testing.T) {
	config := &Config{
		DpiBypass: &DPIBypassConfig{
			Enabled: true,
			ResponseFragmentation: &ResponseFragmentationConfig{
				MaxChunkSize: 15000,
			},
		},
	}

	if !config.IsDPIBypassEnabled() {
		t.Error("DPI bypass should be enabled")
	}

	dpiConfig := config.GetDPIBypassConfig()
	if dpiConfig == nil || !dpiConfig.Enabled {
		t.Error("DPI config should be enabled")
	}

	if dpiConfig.ResponseFragmentation.GetNormalizedMaxChunkSize() != 15000 {
		t.Error("Max chunk size should be 15000")
	}
}

func BenchmarkResponseFragmentation(b *testing.B) {
	config := &ResponseFragmentationConfig{
		Enabled:      true,
		MaxChunkSize: 14000,
		RandomDelay:  &RangeConfig{From: 0, To: 0}, // No delay for benchmarking
	}

	fragmenter := NewResponseFragmenter(config)
	testData := make([]byte, 50000) // 50KB test data

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		ctx := context.Background()
		fragmenter.FragmentResponse(ctx, testData, recorder)
	}
}

func BenchmarkTrafficMasking(b *testing.B) {
	config := &TrafficMaskingConfig{
		UserAgentRotation:         true,
		RefererGeneration:        true,
		CookieManagement:         true,
		BrowserBehaviorSimulation: true,
	}

	masker := NewTrafficMasker(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "https://example.com/test", nil)
		masker.MaskRequest(req)
	}
}

func BenchmarkEntropyReduction(b *testing.B) {
	config := &EntropyReductionConfig{
		Enabled:          true,
		Method:           "structured-padding",
		TargetEntropy:    0.7,
		PatternInjection: true,
	}

	reducer := NewEntropyReducer(config)
	testData := make([]byte, 10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reducer.ReduceEntropy(testData)
	}
}