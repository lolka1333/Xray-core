package internet

import (
	"crypto/rand"
	"math/big"
	"net"
	"syscall"
	"time"
)

// DPIBypassOptions опции для обхода DPI
type DPIBypassOptions struct {
	// TCP опции
	TCPNoDelay       bool // Отключить алгоритм Nagle
	TCPQuickAck      bool // Быстрое подтверждение TCP
	TCPKeepAlive     bool // Keep-alive пакеты
	TCPUserTimeout   int  // Таймаут пользователя в миллисекундах
	
	// Фрагментация
	FragmentSize     int  // Размер фрагмента
	FragmentDelay    int  // Задержка между фрагментами в микросекундах
	
	// TTL манипуляции
	TTLFake          int  // Поддельный TTL для первых пакетов
	TTLReal          int  // Реальный TTL
	
	// Специальные техники
	TCPFastOpen      bool // TCP Fast Open
	TCPSyncookie     bool // SYN cookies
	WindowSize       int  // Размер TCP окна
	MSS              int  // Maximum Segment Size
}

// GetDefaultDPIBypassOptions возвращает оптимальные настройки для обхода российских DPI
// Реализация зависит от платформы (см. dpi_bypass_windows.go и dpi_bypass_unix.go)

// ApplyDPIBypassOptions применяет опции обхода DPI к сокету
func ApplyDPIBypassOptions(fd uintptr, options *DPIBypassOptions) error {
	if options == nil {
		options = GetDefaultDPIBypassOptions()
	}
	
	// TCP_NODELAY - отключаем алгоритм Nagle для уменьшения задержек
	if options.TCPNoDelay {
		if err := setSockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1); err != nil {
			// Игнорируем ошибку, продолжаем применять другие опции
		}
	}
	
	// TCP_QUICKACK - быстрое подтверждение пакетов
	if options.TCPQuickAck {
		// TCP_QUICKACK = 12 (Linux specific)
		setSockoptInt(fd, syscall.IPPROTO_TCP, 12, 1)
	}
	
	// Устанавливаем размер окна
	if options.WindowSize > 0 {
		setSockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, options.WindowSize)
		setSockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_SNDBUF, options.WindowSize)
	}
	
	// TCP_USER_TIMEOUT - таймаут для неподтвержденных данных
	if options.TCPUserTimeout > 0 {
		// TCP_USER_TIMEOUT = 18 (Linux specific)
		setSockoptInt(fd, syscall.IPPROTO_TCP, 18, options.TCPUserTimeout)
	}
	
	// TCP_FASTOPEN - ускоренное открытие соединения
	if options.TCPFastOpen {
		// TCP_FASTOPEN = 23 (Linux specific)
		setSockoptInt(fd, syscall.IPPROTO_TCP, 23, 1)
	}
	
	// MSS - Maximum Segment Size
	if options.MSS > 0 {
		// TCP_MAXSEG = 2
		setSockoptInt(fd, syscall.IPPROTO_TCP, 2, options.MSS)
	}
	
	return nil
}

// DPIBypassConn обертка для соединения с обходом DPI
type DPIBypassConn struct {
	net.Conn
	options       *DPIBypassOptions
	ttlManipulate bool
	firstPacket   bool
}

// NewDPIBypassConn создает новое соединение с обходом DPI
func NewDPIBypassConn(conn net.Conn, options *DPIBypassOptions) *DPIBypassConn {
	if options == nil {
		options = GetDefaultDPIBypassOptions()
	}
	
	return &DPIBypassConn{
		Conn:          conn,
		options:       options,
		ttlManipulate: options.TTLFake > 0 && options.TTLReal > 0,
		firstPacket:   true,
	}
}

// Write отправляет данные с применением техник обхода DPI
func (dbc *DPIBypassConn) Write(b []byte) (int, error) {
	// Применяем TTL манипуляции для первого пакета
	if dbc.ttlManipulate && dbc.firstPacket {
		dbc.firstPacket = false
		return dbc.writeTTLManipulated(b)
	}
	
	// Применяем фрагментацию
	if dbc.options.FragmentSize > 0 && len(b) > dbc.options.FragmentSize {
		return dbc.writeFragmented(b)
	}
	
	return dbc.Conn.Write(b)
}

// writeTTLManipulated отправляет данные с TTL манипуляциями
func (dbc *DPIBypassConn) writeTTLManipulated(data []byte) (int, error) {
	// Техника: отправляем первые байты с низким TTL (не дойдут до сервера)
	// DPI увидит "неправильные" данные и пропустит соединение
	
	if tcpConn, ok := dbc.Conn.(*net.TCPConn); ok {
		// Получаем файловый дескриптор
		rawConn, err := tcpConn.SyscallConn()
		if err == nil {
			var setTTLErr error
			
			// Устанавливаем низкий TTL
			rawConn.Control(func(fd uintptr) {
				setTTLErr = setSockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_TTL, dbc.options.TTLFake)
			})
			
			if setTTLErr == nil {
				// Отправляем поддельный пакет с мусором
				fakeData := make([]byte, min(len(data), 5))
				rand.Read(fakeData)
				dbc.Conn.Write(fakeData)
				
				// Небольшая задержка
				time.Sleep(time.Microsecond * 100)
				
				// Восстанавливаем нормальный TTL
				rawConn.Control(func(fd uintptr) {
					setSockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_TTL, dbc.options.TTLReal)
				})
			}
		}
	}
	
	// Отправляем реальные данные
	return dbc.writeFragmented(data)
}

// writeFragmented отправляет данные фрагментами
func (dbc *DPIBypassConn) writeFragmented(data []byte) (int, error) {
	totalWritten := 0
	dataLen := len(data)
	
	// Специальная обработка для TLS ClientHello
	if dataLen > 5 && data[0] == 0x16 && data[5] == 0x01 {
		// Отправляем TLS заголовок отдельно
		n, err := dbc.Conn.Write(data[:5])
		if err != nil {
			return n, err
		}
		totalWritten += n
		
		// Задержка
		if dbc.options.FragmentDelay > 0 {
			time.Sleep(time.Microsecond * time.Duration(dbc.options.FragmentDelay))
		}
		
		// Отправляем остальное маленькими частями
		for totalWritten < dataLen {
			chunkSize := dbc.options.FragmentSize
			if totalWritten+chunkSize > dataLen {
				chunkSize = dataLen - totalWritten
			}
			
			// Для области SNI (примерно 40-100 байт) используем еще меньшие фрагменты
			if totalWritten > 40 && totalWritten < 100 {
				chunkSize = min(chunkSize, 3+randInt(0, 5))
			}
			
			n, err := dbc.Conn.Write(data[totalWritten : totalWritten+chunkSize])
			if err != nil {
				return totalWritten, err
			}
			totalWritten += n
			
			// Случайная задержка между фрагментами
			if totalWritten < dataLen && dbc.options.FragmentDelay > 0 {
				delay := dbc.options.FragmentDelay + randInt(0, dbc.options.FragmentDelay/2)
				time.Sleep(time.Microsecond * time.Duration(delay))
			}
		}
		
		return dataLen, nil
	}
	
	// Обычная фрагментация для других данных
	for totalWritten < dataLen {
		chunkSize := dbc.options.FragmentSize
		if totalWritten+chunkSize > dataLen {
			chunkSize = dataLen - totalWritten
		}
		
		n, err := dbc.Conn.Write(data[totalWritten : totalWritten+chunkSize])
		if err != nil {
			return totalWritten, err
		}
		totalWritten += n
		
		if totalWritten < dataLen && dbc.options.FragmentDelay > 0 {
			time.Sleep(time.Microsecond * time.Duration(dbc.options.FragmentDelay))
		}
	}
	
	return totalWritten, nil
}

// ApplyDPIBypassToSocket применяет техники обхода DPI к существующему сокету
func ApplyDPIBypassToSocket(conn net.Conn) error {
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		rawConn, err := tcpConn.SyscallConn()
		if err != nil {
			return err
		}
		
		options := GetDefaultDPIBypassOptions()
		
		return rawConn.Control(func(fd uintptr) {
			ApplyDPIBypassOptions(fd, options)
		})
	}
	
	return nil
}

// randInt генерирует случайное число в диапазоне
func randInt(min, max int) int {
	if min >= max {
		return min
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max-min)))
	return int(n.Int64()) + min
}

// min возвращает минимальное из двух чисел
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}