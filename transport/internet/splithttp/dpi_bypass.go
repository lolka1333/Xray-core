package splithttp

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xtls/xray-core/common/errors"
)

// ResponseFragmenter handles breaking large responses into smaller chunks
// to bypass Russian DPI 15-20KB connection limits
type ResponseFragmenter struct {
	config *ResponseFragmentationConfig
	mu     sync.RWMutex
}

// NewResponseFragmenter creates a new fragmenter with given config
func NewResponseFragmenter(config *ResponseFragmentationConfig) *ResponseFragmenter {
	if config == nil {
		config = &ResponseFragmentationConfig{
			Enabled:       true,
			MaxChunkSize:  14000, // Stay under 15KB limit
			RandomDelay:   &RangeConfig{From: 100, To: 500},
			ConnectionPooling: true,
		}
	}
	return &ResponseFragmenter{config: config}
}

// FragmentResponse breaks a large response into multiple chunks
func (rf *ResponseFragmenter) FragmentResponse(ctx context.Context, data []byte, writer http.ResponseWriter) error {
	if !rf.config.Enabled || len(data) <= int(rf.config.MaxChunkSize) {
		_, err := writer.Write(data)
		return err
	}

	rf.mu.RLock()
	defer rf.mu.RUnlock()

	chunkSize := int(rf.config.MaxChunkSize)
	chunks := make([][]byte, 0, (len(data)+chunkSize-1)/chunkSize)

	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunks = append(chunks, data[i:end])
	}

	// Send first chunk immediately
	if len(chunks) > 0 {
		if _, err := writer.Write(chunks[0]); err != nil {
			return err
		}
		writer.(http.Flusher).Flush()
	}

	// Send remaining chunks with delays
	for i := 1; i < len(chunks); i++ {
		// Add random delay to simulate natural behavior
		delay := rf.getRandomDelay()
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		if _, err := writer.Write(chunks[i]); err != nil {
			return err
		}
		writer.(http.Flusher).Flush()
	}

	return nil
}

func (rf *ResponseFragmenter) getRandomDelay() time.Duration {
	if rf.config.RandomDelay == nil {
		return 0
	}
	min := rf.config.RandomDelay.From
	max := rf.config.RandomDelay.To
	if min >= max {
		return time.Duration(min) * time.Millisecond
	}
	
	diff := max - min
	randVal := make([]byte, 1)
	rand.Read(randVal)
	delay := min + int32(randVal[0])%diff
	
	return time.Duration(delay) * time.Millisecond
}

// ConnectionManager handles connection rotation and pooling
type ConnectionManager struct {
	config *ConnectionManagementConfig
	activeConnections map[string]*ManagedConnection
	mu sync.RWMutex
	currentIndex int
}

type ManagedConnection struct {
	conn      io.ReadWriteCloser
	createdAt time.Time
	requestCount int
	lastUsed  time.Time
}

func NewConnectionManager(config *ConnectionManagementConfig) *ConnectionManager {
	if config == nil {
		config = &ConnectionManagementConfig{
			MaxConnectionLifetime: 30,
			ConnectionRotation:    true,
			ParallelConnections:   3,
			LoadBalancing:        "round-robin",
		}
	}
	return &ConnectionManager{
		config:            config,
		activeConnections: make(map[string]*ManagedConnection),
	}
}

func (cm *ConnectionManager) GetConnection(ctx context.Context, createConn func() (io.ReadWriteCloser, error)) (io.ReadWriteCloser, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Clean up expired connections
	cm.cleanupExpiredConnections()

	// Find available connection using load balancing
	connId := cm.selectConnection()
	
	if managedConn, exists := cm.activeConnections[connId]; exists {
		managedConn.lastUsed = time.Now()
		managedConn.requestCount++
		return managedConn.conn, nil
	}

	// Create new connection
	conn, err := createConn()
	if err != nil {
		return nil, err
	}

	cm.activeConnections[connId] = &ManagedConnection{
		conn:      conn,
		createdAt: time.Now(),
		requestCount: 1,
		lastUsed:  time.Now(),
	}

	return conn, nil
}

func (cm *ConnectionManager) selectConnection() string {
	switch cm.config.LoadBalancing {
	case "round-robin":
		cm.currentIndex = (cm.currentIndex + 1) % int(cm.config.ParallelConnections)
		return fmt.Sprintf("conn_%d", cm.currentIndex)
	case "random":
		randBytes := make([]byte, 1)
		rand.Read(randBytes)
		index := int(randBytes[0]) % int(cm.config.ParallelConnections)
		return fmt.Sprintf("conn_%d", index)
	default:
		return "conn_0"
	}
}

func (cm *ConnectionManager) cleanupExpiredConnections() {
	maxLifetime := time.Duration(cm.config.MaxConnectionLifetime) * time.Second
	now := time.Now()

	for id, conn := range cm.activeConnections {
		if now.Sub(conn.createdAt) > maxLifetime {
			conn.conn.Close()
			delete(cm.activeConnections, id)
		}
	}
}

// TrafficMasker handles mimicking legitimate HTTP traffic
type TrafficMasker struct {
	config *TrafficMaskingConfig
	userAgents []string
	mu sync.RWMutex
}

func NewTrafficMasker(config *TrafficMaskingConfig) *TrafficMasker {
	if config == nil {
		config = &TrafficMaskingConfig{
			UserAgentRotation: true,
			AcceptHeaders: []string{"text/html", "application/json", "image/*"},
			RefererGeneration: true,
			CookieManagement: true,
			CompressionSupport: []string{"gzip", "deflate", "br"},
			BrowserBehaviorSimulation: true,
		}
	}

	masker := &TrafficMasker{
		config: config,
		userAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		},
	}
	
	return masker
}

func (tm *TrafficMasker) MaskRequest(req *http.Request) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if tm.config.UserAgentRotation {
		req.Header.Set("User-Agent", tm.getRandomUserAgent())
	}

	if tm.config.RefererGeneration {
		referer := tm.generateReferer(req.URL.Host)
		if referer != "" {
			req.Header.Set("Referer", referer)
		}
	}

	// Set common browser headers
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.8")
	req.Header.Set("Accept-Encoding", strings.Join(tm.config.CompressionSupport, ", "))
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	
	if len(tm.config.AcceptHeaders) > 0 {
		req.Header.Set("Accept", tm.config.AcceptHeaders[0])
	}

	if tm.config.CookieManagement {
		tm.addCookies(req)
	}
}

func (tm *TrafficMasker) getRandomUserAgent() string {
	if len(tm.userAgents) == 0 {
		return tm.userAgents[0]
	}
	randBytes := make([]byte, 1)
	rand.Read(randBytes)
	index := int(randBytes[0]) % len(tm.userAgents)
	return tm.userAgents[index]
}

func (tm *TrafficMasker) generateReferer(host string) string {
	commonReferrers := []string{
		"https://www.google.com/",
		"https://yandex.ru/",
		"https://www.bing.com/",
		"https://" + host + "/",
	}
	
	randBytes := make([]byte, 1)
	rand.Read(randBytes)
	index := int(randBytes[0]) % len(commonReferrers)
	return commonReferrers[index]
}

func (tm *TrafficMasker) addCookies(req *http.Request) {
	// Add some common cookies to look more legitimate
	sessionId := tm.generateSessionId()
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionId})
	req.AddCookie(&http.Cookie{Name: "lang", Value: "ru"})
	req.AddCookie(&http.Cookie{Name: "_ga", Value: "GA1.2." + tm.generateSessionId()})
}

func (tm *TrafficMasker) generateSessionId() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	rand.Read(b)
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}

// CDNHopper handles switching between different CDN providers
type CDNHopper struct {
	config *CDNHoppingConfig
	currentProvider int
	failoverCount map[string]int
	lastRotation time.Time
	mu sync.RWMutex
}

func NewCDNHopper(config *CDNHoppingConfig) *CDNHopper {
	if config == nil {
		config = &CDNHoppingConfig{
			Enabled: true,
			Providers: []string{"cloudflare", "gcore", "aws"},
			RotationInterval: 60,
			FailoverThreshold: 2,
		}
	}
	
	return &CDNHopper{
		config:        config,
		failoverCount: make(map[string]int),
		lastRotation:  time.Now(),
	}
}

func (ch *CDNHopper) GetCurrentEndpoint(baseHost string) string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	if !ch.config.Enabled || len(ch.config.Providers) == 0 {
		return baseHost
	}

	provider := ch.config.Providers[ch.currentProvider]
	return ch.getEndpointForProvider(provider, baseHost)
}

func (ch *CDNHopper) getEndpointForProvider(provider, baseHost string) string {
	switch provider {
	case "cloudflare":
		return baseHost // Cloudflare uses original domain
	case "gcore":
		return baseHost // GCore uses original domain  
	case "aws":
		return baseHost // AWS CloudFront uses original domain
	default:
		return baseHost
	}
}

func (ch *CDNHopper) HandleFailover(provider string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	ch.failoverCount[provider]++
	
	if ch.failoverCount[provider] >= int(ch.config.FailoverThreshold) {
		ch.rotateProvider()
		ch.failoverCount[provider] = 0
	}
}

func (ch *CDNHopper) ShouldRotate() bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	interval := time.Duration(ch.config.RotationInterval) * time.Second
	return time.Since(ch.lastRotation) > interval
}

func (ch *CDNHopper) rotateProvider() {
	if len(ch.config.Providers) <= 1 {
		return
	}
	
	ch.currentProvider = (ch.currentProvider + 1) % len(ch.config.Providers)
	ch.lastRotation = time.Now()
	
	errors.LogInfo(context.Background(), "CDN hopping: switched to provider", ch.config.Providers[ch.currentProvider])
}

// EntropyReducer handles reducing entropy in traffic to avoid detection
type EntropyReducer struct {
	config *EntropyReductionConfig
}

func NewEntropyReducer(config *EntropyReductionConfig) *EntropyReducer {
	if config == nil {
		config = &EntropyReductionConfig{
			Enabled:         true,
			Method:          "structured-padding",
			TargetEntropy:   0.7,
			PatternInjection: true,
		}
	}
	
	return &EntropyReducer{config: config}
}

func (er *EntropyReducer) ReduceEntropy(data []byte) []byte {
	if !er.config.Enabled {
		return data
	}

	entropy := er.calculateEntropy(data)
	
	if entropy > er.config.TargetEntropy {
		return er.injectStructuredPadding(data)
	}
	
	return data
}

func (er *EntropyReducer) calculateEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}

	freq := make(map[byte]int)
	
	for _, b := range data {
		freq[b]++
	}
	
	entropy := 0.0
	length := float64(len(data))
	
	for _, count := range freq {
		p := float64(count) / length
		if p > 0 {
			entropy += p * math.Log2(p)
		}
	}
	
	return -entropy
}

func (er *EntropyReducer) injectStructuredPadding(data []byte) []byte {
	if er.config.Method != "structured-padding" {
		return data
	}

	patterns := [][]byte{
		[]byte("padding123456789"),
		[]byte("abcdefghijklmnop"),
		[]byte("1234567890123456"),
		[]byte("________________"), // underscores
		[]byte("................"), // dots
	}
	
	result := make([]byte, 0, len(data)*2)
	
	randBytes := make([]byte, 1)
	rand.Read(randBytes)
	pattern := patterns[int(randBytes[0])%len(patterns)]
	
	// Insert structured padding every 1KB
	for i := 0; i < len(data); i += 1024 {
		end := i + 1024
		if end > len(data) {
			end = len(data)
		}
		
		result = append(result, data[i:end]...)
		
		// Insert padding if not at the end
		if end < len(data) {
			result = append(result, pattern...)
		}
	}
	
	return result
}

// DPIBypassManager coordinates all DPI bypass techniques
type DPIBypassManager struct {
	config              *DPIBypassConfig
	responseFragmenter  *ResponseFragmenter
	connectionManager   *ConnectionManager
	trafficMasker       *TrafficMasker
	cdnHopper          *CDNHopper
	entropyReducer     *EntropyReducer
}

func NewDPIBypassManager(config *DPIBypassConfig) *DPIBypassManager {
	if config == nil || !config.Enabled {
		return &DPIBypassManager{config: &DPIBypassConfig{Enabled: false}}
	}

	return &DPIBypassManager{
		config:             config,
		responseFragmenter: NewResponseFragmenter(config.ResponseFragmentation),
		connectionManager:  NewConnectionManager(config.ConnectionManagement),
		trafficMasker:     NewTrafficMasker(config.TrafficMasking),
		cdnHopper:         NewCDNHopper(config.CdnHopping),
		entropyReducer:    NewEntropyReducer(config.EntropyReduction),
	}
}

func (dm *DPIBypassManager) IsEnabled() bool {
	return dm.config != nil && dm.config.Enabled
}

func (dm *DPIBypassManager) ProcessRequest(req *http.Request) {
	if !dm.IsEnabled() {
		return
	}

	if dm.trafficMasker != nil {
		dm.trafficMasker.MaskRequest(req)
	}

	if dm.cdnHopper != nil && dm.cdnHopper.ShouldRotate() {
		dm.cdnHopper.rotateProvider()
	}
}

func (dm *DPIBypassManager) ProcessResponseData(ctx context.Context, data []byte, writer http.ResponseWriter) error {
	if !dm.IsEnabled() {
		_, err := writer.Write(data)
		return err
	}

	// Apply entropy reduction
	if dm.entropyReducer != nil {
		data = dm.entropyReducer.ReduceEntropy(data)
	}

	// Fragment response if needed
	if dm.responseFragmenter != nil {
		return dm.responseFragmenter.FragmentResponse(ctx, data, writer)
	}

	_, err := writer.Write(data)
	return err
}

func (dm *DPIBypassManager) GetManagedConnection(ctx context.Context, createConn func() (io.ReadWriteCloser, error)) (io.ReadWriteCloser, error) {
	if !dm.IsEnabled() || dm.connectionManager == nil {
		return createConn()
	}

	return dm.connectionManager.GetConnection(ctx, createConn)
}

func (dm *DPIBypassManager) GetEndpoint(baseHost string) string {
	if !dm.IsEnabled() || dm.cdnHopper == nil {
		return baseHost
	}

	return dm.cdnHopper.GetCurrentEndpoint(baseHost)
}