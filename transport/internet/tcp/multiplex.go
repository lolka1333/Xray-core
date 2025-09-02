package tcp

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"

	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xtls/xray-core/common/errors"

)

const (
	// Лимит данных на одно TCP соединение (15KB для обхода блокировки РКН)
	DefaultDataLimitPerConnection = 15 * 1024 // 15KB
	
	// Минимальный лимит для безопасности
	MinDataLimit = 10 * 1024 // 10KB
	
	// Максимальное количество параллельных соединений
	MaxConcurrentConnections = 16
	
	// Таймаут для создания нового соединения
	ConnectionTimeout = 10 * time.Second
)

// MultiplexConfig конфигурация мультиплексирования
type MultiplexConfig struct {
	DataLimitPerConnection int64  // Лимит данных на соединение в байтах
	MaxConnections        int    // Максимум параллельных соединений
	RotateOnLimit         bool   // Автоматическая ротация при достижении лимита
	MaskAsSSH             bool   // Маскировка под SSH трафик
	AdaptiveLimit         bool   // Адаптивное определение лимита провайдера
}

// GetDefaultMultiplexConfig возвращает конфигурацию по умолчанию
func GetDefaultMultiplexConfig() *MultiplexConfig {
	return &MultiplexConfig{
		DataLimitPerConnection: DefaultDataLimitPerConnection,
		MaxConnections:        8,
		RotateOnLimit:         true,
		MaskAsSSH:             false,
		AdaptiveLimit:         true,
	}
}

// MultiplexedConn мультиплексированное соединение
type MultiplexedConn struct {
	config         *MultiplexConfig
	dialer         func() (net.Conn, error)
	connections    []*trackedConn
	currentConn    *trackedConn
	mu             sync.RWMutex
	closed         bool
	ctx            context.Context
	cancel         context.CancelFunc
	rotationCount  int32
}

// trackedConn отслеживаемое соединение
type trackedConn struct {
	conn         net.Conn
	bytesRead    int64
	bytesWritten int64
	created      time.Time
	lastUsed     time.Time
	id           int
}

// NewMultiplexedConn создает новое мультиплексированное соединение
func NewMultiplexedConn(ctx context.Context, dialer func() (net.Conn, error), config *MultiplexConfig) (*MultiplexedConn, error) {
	if config == nil {
		config = GetDefaultMultiplexConfig()
	}
	
	// Адаптивное определение лимита провайдера
	if config.AdaptiveLimit {
		detectedLimit := detectProviderLimit(ctx, dialer)
		if detectedLimit > 0 {
			config.DataLimitPerConnection = detectedLimit
			errors.LogInfo(ctx, fmt.Sprintf("Detected provider limit: %d bytes", detectedLimit))
		}
	}
	
	ctx, cancel := context.WithCancel(ctx)
	
	mc := &MultiplexedConn{
		config:      config,
		dialer:      dialer,
		connections: make([]*trackedConn, 0, config.MaxConnections),
		ctx:         ctx,
		cancel:      cancel,
	}
	
	// Создаем первое соединение
	conn, err := mc.createNewConnection()
	if err != nil {
		cancel()
		return nil, err
	}
	
	mc.currentConn = conn
	mc.connections = append(mc.connections, conn)
	
	// Запускаем мониторинг соединений
	go mc.monitor()
	
	return mc, nil
}

// createNewConnection создает новое TCP соединение
func (mc *MultiplexedConn) createNewConnection() (*trackedConn, error) {
	conn, err := mc.dialer()
	if err != nil {
		return nil, err
	}
	
	// Применяем маскировку под SSH если нужно
	if mc.config.MaskAsSSH {
		conn = newSSHMaskedConn(conn)
	}
	
	tracked := &trackedConn{
		conn:     conn,
		created:  time.Now(),
		lastUsed: time.Now(),
		id:       int(atomic.AddInt32(&mc.rotationCount, 1)),
	}
	
	errors.LogDebug(mc.ctx, fmt.Sprintf("Created new connection #%d", tracked.id))
	
	return tracked, nil
}

// Read читает данные с автоматической ротацией соединений
func (mc *MultiplexedConn) Read(b []byte) (int, error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	if mc.closed {
		return 0, io.EOF
	}
	
	// Проверяем, нужна ли ротация
	if mc.shouldRotate(mc.currentConn, int64(len(b))) {
		if err := mc.rotateConnection(); err != nil {
			return 0, err
		}
	}
	
	n, err := mc.currentConn.conn.Read(b)
	if n > 0 {
		atomic.AddInt64(&mc.currentConn.bytesRead, int64(n))
		mc.currentConn.lastUsed = time.Now()
		
		// Логируем прогресс
		if atomic.LoadInt64(&mc.currentConn.bytesRead)%5000 == 0 {
			errors.LogDebug(mc.ctx, fmt.Sprintf("Connection #%d: read %d bytes (limit: %d)",
				mc.currentConn.id,
				atomic.LoadInt64(&mc.currentConn.bytesRead),
				mc.config.DataLimitPerConnection))
		}
	}
	
	return n, err
}

// Write записывает данные с автоматической ротацией соединений
func (mc *MultiplexedConn) Write(b []byte) (int, error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	if mc.closed {
		return 0, io.EOF
	}
	
	totalWritten := 0
	data := b
	
	for len(data) > 0 {
		// Определяем, сколько можем записать в текущее соединение
		remainingCapacity := mc.config.DataLimitPerConnection - atomic.LoadInt64(&mc.currentConn.bytesWritten)
		if remainingCapacity <= 0 {
			if err := mc.rotateConnection(); err != nil {
				return totalWritten, err
			}
			remainingCapacity = mc.config.DataLimitPerConnection
		}
		
		// Записываем порцию данных
		toWrite := int64(len(data))
		if toWrite > remainingCapacity {
			toWrite = remainingCapacity
		}
		
		n, err := mc.currentConn.conn.Write(data[:toWrite])
		if err != nil {
			return totalWritten, err
		}
		
		atomic.AddInt64(&mc.currentConn.bytesWritten, int64(n))
		mc.currentConn.lastUsed = time.Now()
		totalWritten += n
		data = data[n:]
		
		// Логируем прогресс
		if atomic.LoadInt64(&mc.currentConn.bytesWritten)%5000 == 0 {
			errors.LogDebug(mc.ctx, fmt.Sprintf("Connection #%d: written %d bytes (limit: %d)",
				mc.currentConn.id,
				atomic.LoadInt64(&mc.currentConn.bytesWritten),
				mc.config.DataLimitPerConnection))
		}
	}
	
	return totalWritten, nil
}

// shouldRotate проверяет, нужна ли ротация соединения
func (mc *MultiplexedConn) shouldRotate(conn *trackedConn, nextBytes int64) bool {
	if !mc.config.RotateOnLimit {
		return false
	}
	
	totalBytes := atomic.LoadInt64(&conn.bytesRead) + atomic.LoadInt64(&conn.bytesWritten)
	
	// Оставляем небольшой запас (95% от лимита)
	threshold := int64(float64(mc.config.DataLimitPerConnection) * 0.95)
	
	return totalBytes+nextBytes > threshold
}

// rotateConnection переключается на новое соединение
func (mc *MultiplexedConn) rotateConnection() error {
	errors.LogInfo(mc.ctx, fmt.Sprintf("Rotating connection (current #%d used %d bytes)",
		mc.currentConn.id,
		atomic.LoadInt64(&mc.currentConn.bytesRead)+atomic.LoadInt64(&mc.currentConn.bytesWritten)))
	
	// Ищем существующее соединение с доступной емкостью
	for _, conn := range mc.connections {
		if conn != mc.currentConn {
			totalBytes := atomic.LoadInt64(&conn.bytesRead) + atomic.LoadInt64(&conn.bytesWritten)
			if totalBytes < int64(float64(mc.config.DataLimitPerConnection)*0.8) {
				mc.currentConn = conn
				errors.LogDebug(mc.ctx, fmt.Sprintf("Reusing connection #%d", conn.id))
				return nil
			}
		}
	}
	
	// Создаем новое соединение если не нашли подходящее
	if len(mc.connections) >= mc.config.MaxConnections {
		// Закрываем самое старое соединение
		oldest := mc.connections[0]
		oldest.conn.Close()
		mc.connections = mc.connections[1:]
		errors.LogDebug(mc.ctx, fmt.Sprintf("Closed oldest connection #%d", oldest.id))
	}
	
	newConn, err := mc.createNewConnection()
	if err != nil {
		return err
	}
	
	mc.connections = append(mc.connections, newConn)
	mc.currentConn = newConn
	
	return nil
}

// monitor мониторит состояние соединений
func (mc *MultiplexedConn) monitor() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-mc.ctx.Done():
			return
		case <-ticker.C:
			mc.cleanupIdleConnections()
		}
	}
}

// cleanupIdleConnections закрывает неиспользуемые соединения
func (mc *MultiplexedConn) cleanupIdleConnections() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	now := time.Now()
	activeConnections := make([]*trackedConn, 0, len(mc.connections))
	
	for _, conn := range mc.connections {
		// Закрываем соединения, неиспользуемые более 60 секунд
		if conn != mc.currentConn && now.Sub(conn.lastUsed) > 60*time.Second {
			conn.conn.Close()
			errors.LogDebug(mc.ctx, fmt.Sprintf("Closed idle connection #%d", conn.id))
		} else {
			activeConnections = append(activeConnections, conn)
		}
	}
	
	mc.connections = activeConnections
}

// Close закрывает все соединения
func (mc *MultiplexedConn) Close() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	if mc.closed {
		return nil
	}
	
	mc.closed = true
	mc.cancel()
	
	var lastErr error
	for _, conn := range mc.connections {
		if err := conn.conn.Close(); err != nil {
			lastErr = err
		}
	}
	
	return lastErr
}

// Реализация остальных методов net.Conn
func (mc *MultiplexedConn) LocalAddr() net.Addr {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	if mc.currentConn != nil {
		return mc.currentConn.conn.LocalAddr()
	}
	return nil
}

func (mc *MultiplexedConn) RemoteAddr() net.Addr {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	if mc.currentConn != nil {
		return mc.currentConn.conn.RemoteAddr()
	}
	return nil
}

func (mc *MultiplexedConn) SetDeadline(t time.Time) error {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	for _, conn := range mc.connections {
		conn.conn.SetDeadline(t)
	}
	return nil
}

func (mc *MultiplexedConn) SetReadDeadline(t time.Time) error {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	for _, conn := range mc.connections {
		conn.conn.SetReadDeadline(t)
	}
	return nil
}

func (mc *MultiplexedConn) SetWriteDeadline(t time.Time) error {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	for _, conn := range mc.connections {
		conn.conn.SetWriteDeadline(t)
	}
	return nil
}

// detectProviderLimit определяет лимит провайдера экспериментально
func detectProviderLimit(ctx context.Context, dialer func() (net.Conn, error)) int64 {
	// Простая эвристика: начинаем с 20KB и уменьшаем
	limits := []int64{20 * 1024, 18 * 1024, 15 * 1024, 12 * 1024, 10 * 1024}
	
	for _, limit := range limits {
		if testLimit(ctx, dialer, limit) {
			// Возвращаем 90% от рабочего лимита для безопасности
			return int64(float64(limit) * 0.9)
		}
	}
	
	// По умолчанию используем самый безопасный вариант
	return MinDataLimit
}

// testLimit тестирует конкретный лимит
func testLimit(ctx context.Context, dialer func() (net.Conn, error), limit int64) bool {
	conn, err := dialer()
	if err != nil {
		return false
	}
	defer conn.Close()
	
	// Пытаемся передать данные размером с лимит
	testData := make([]byte, limit)
	rand.Read(testData)
	
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write(testData)
	
	return err == nil
}

// sshMaskedConn маскирует трафик под SSH
type sshMaskedConn struct {
	net.Conn
	firstWrite bool
	firstRead  bool
}

// newSSHMaskedConn создает соединение с маскировкой под SSH
func newSSHMaskedConn(conn net.Conn) net.Conn {
	return &sshMaskedConn{
		Conn:       conn,
		firstWrite: true,
		firstRead:  true,
	}
}

// Write отправляет данные с SSH заголовком
func (sc *sshMaskedConn) Write(b []byte) (int, error) {
	if sc.firstWrite {
		sc.firstWrite = false
		// Отправляем SSH-like заголовок
		sshHeader := []byte("SSH-2.0-OpenSSH_8.9p1 Ubuntu-3ubuntu0.1\r\n")
		if _, err := sc.Conn.Write(sshHeader); err != nil {
			return 0, err
		}
		// Небольшая задержка как в реальном SSH
		time.Sleep(time.Millisecond * time.Duration(randInt(10, 50)))
	}
	
	return sc.Conn.Write(b)
}

// Read читает данные, пропуская SSH заголовки
func (sc *sshMaskedConn) Read(b []byte) (int, error) {
	if sc.firstRead {
		sc.firstRead = false
		// Читаем и игнорируем SSH заголовок сервера
		header := make([]byte, 255)
		n, err := sc.Conn.Read(header)
		if err != nil {
			return 0, err
		}
		// Ищем конец заголовка
		for i := 0; i < n-1; i++ {
			if header[i] == '\r' && header[i+1] == '\n' {
				// Нашли конец заголовка
				break
			}
		}
	}
	
	return sc.Conn.Read(b)
}

