package tcp

import (
	"bytes"
	"crypto/rand"
	"io"
	"math/big"
	"net"
	"time"
)

// FragmentWriter обертка для фрагментации TCP пакетов
type FragmentWriter struct {
	writer          io.Writer
	conn            net.Conn
	fragmentSize    int
	randomDelay     bool
	paddingEnabled  bool
	splitFirstPacket bool
}

// NewFragmentWriter создает новый FragmentWriter
func NewFragmentWriter(conn net.Conn, fragmentSize int) *FragmentWriter {
	return &FragmentWriter{
		writer:          conn,
		conn:            conn,
		fragmentSize:    fragmentSize,
		randomDelay:     true,
		paddingEnabled:  true,
		splitFirstPacket: true,
	}
}

// Write фрагментирует и отправляет данные
func (fw *FragmentWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	// Специальная обработка первого пакета (TLS ClientHello или HTTP запроса)
	if fw.splitFirstPacket && isTLSClientHello(data) {
		return fw.writeTLSFragmented(data)
	}

	// Обычная фрагментация
	totalWritten := 0
	for totalWritten < len(data) {
		end := totalWritten + fw.fragmentSize
		if end > len(data) {
			end = len(data)
		}

		chunk := data[totalWritten:end]
		
		// Добавляем padding для маскировки
		if fw.paddingEnabled && len(chunk) < fw.fragmentSize {
			chunk = fw.addPadding(chunk)
		}

		n, err := fw.writer.Write(chunk)
		if err != nil {
			return totalWritten, err
		}
		totalWritten += n

		// Случайная задержка между фрагментами
		if fw.randomDelay && totalWritten < len(data) {
			fw.randomSleep()
		}
	}

	return totalWritten, nil
}

// writeTLSFragmented специальная фрагментация для TLS ClientHello
func (fw *FragmentWriter) writeTLSFragmented(data []byte) (int, error) {
	// Разбиваем ClientHello на части для обхода DPI
	// Первый фрагмент - только TLS заголовок (5 байт)
	if len(data) > 5 {
		// Отправляем первые 5 байт (TLS заголовок)
		_, err := fw.writer.Write(data[:5])
		if err != nil {
			return 0, err
		}
		
		// Небольшая задержка
		time.Sleep(time.Millisecond * time.Duration(randInt(10, 50)))
		
		// Разбиваем SNI на части
		if sniPos := findSNI(data); sniPos > 0 {
			// Отправляем до SNI
			if sniPos > 5 {
				_, err = fw.writer.Write(data[5:sniPos])
				if err != nil {
					return 0, err
				}
				time.Sleep(time.Millisecond * time.Duration(randInt(5, 20)))
			}
			
			// Фрагментируем SNI
			sniEnd := sniPos + 20 // Примерная длина SNI
			if sniEnd > len(data) {
				sniEnd = len(data)
			}
			
			// Отправляем SNI по частям
			for i := sniPos; i < sniEnd; i += 3 {
				end := i + 3
				if end > sniEnd {
					end = sniEnd
				}
				_, err = fw.writer.Write(data[i:end])
				if err != nil {
					return 0, err
				}
				if end < sniEnd {
					time.Sleep(time.Millisecond * time.Duration(randInt(2, 10)))
				}
			}
			
			// Отправляем остаток
			if sniEnd < len(data) {
				_, err = fw.writer.Write(data[sniEnd:])
				if err != nil {
					return 0, err
				}
			}
		} else {
			// Если SNI не найден, просто отправляем остаток
			_, err = fw.writer.Write(data[5:])
			if err != nil {
				return 0, err
			}
		}
		
		return len(data), nil
	}
	
	return fw.writer.Write(data)
}

// isTLSClientHello проверяет, является ли пакет TLS ClientHello
func isTLSClientHello(data []byte) bool {
	if len(data) < 6 {
		return false
	}
	// TLS handshake: 0x16 (handshake), версия TLS, и тип ClientHello (0x01)
	return data[0] == 0x16 && data[5] == 0x01
}

// findSNI находит позицию SNI в ClientHello
func findSNI(data []byte) int {
	// Упрощенный поиск SNI extension (тип 0x0000)
	for i := 43; i < len(data)-5; i++ {
		if data[i] == 0x00 && data[i+1] == 0x00 {
			// Проверяем, что это действительно SNI
			if i+4 < len(data) && data[i+4] == 0x00 {
				return i
			}
		}
	}
	return -1
}

// addPadding добавляет случайный padding к данным
func (fw *FragmentWriter) addPadding(data []byte) []byte {
	paddingSize := randInt(1, 32)
	padding := make([]byte, paddingSize)
	rand.Read(padding)
	
	// Создаем буфер с padding
	buf := bytes.NewBuffer(nil)
	buf.Write(data)
	// Padding добавляется, но игнорируется на принимающей стороне
	
	return data // Возвращаем оригинальные данные, так как padding должен быть на уровне протокола
}

// randomSleep делает случайную паузу
func (fw *FragmentWriter) randomSleep() {
	delay := randInt(1, 10)
	time.Sleep(time.Microsecond * time.Duration(delay))
}

// randInt генерирует случайное число в диапазоне
func randInt(min, max int64) int64 {
	if min >= max {
		return min
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(max-min))
	return n.Int64() + min
}

// FragmentConn обертка для net.Conn с фрагментацией
type FragmentConn struct {
	net.Conn
	writer *FragmentWriter
}

// NewFragmentConn создает новое соединение с фрагментацией
func NewFragmentConn(conn net.Conn, fragmentSize int) *FragmentConn {
	return &FragmentConn{
		Conn:   conn,
		writer: NewFragmentWriter(conn, fragmentSize),
	}
}

// Write отправляет данные с фрагментацией
func (fc *FragmentConn) Write(b []byte) (int, error) {
	return fc.writer.Write(b)
}