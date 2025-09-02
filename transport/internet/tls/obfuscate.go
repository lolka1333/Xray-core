package tls

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"math/big"
	"time"
)

// ObfuscatedConn обертка для обфускации TLS трафика
type ObfuscatedConn struct {
	conn           Interface
	obfuscateSNI   bool
	paddingEnabled bool
	splitHandshake bool
}

// NewObfuscatedConn создает новое соединение с обфускацией
func NewObfuscatedConn(conn Interface) *ObfuscatedConn {
	return &ObfuscatedConn{
		conn:           conn,
		obfuscateSNI:   true,
		paddingEnabled: true,
		splitHandshake: true,
	}
}

// HandshakeContext выполняет обфусцированный TLS handshake
func (oc *ObfuscatedConn) HandshakeContext(ctx context.Context) error {
	// Добавляем случайную задержку перед handshake
	randomDelay()
	
	// Выполняем оригинальный handshake
	return oc.conn.HandshakeContext(ctx)
}

// ObfuscateClientHello модифицирует ClientHello для обхода DPI
func ObfuscateClientHello(data []byte) []byte {
	if !isTLSHandshake(data) {
		return data
	}
	
	// Добавляем случайные расширения
	data = addRandomExtensions(data)
	
	// Добавляем padding
	data = addTLSPadding(data)
	
	// Перемешиваем порядок расширений
	data = shuffleExtensions(data)
	
	return data
}

// addRandomExtensions добавляет случайные TLS расширения
func addRandomExtensions(data []byte) []byte {
	// Добавляем безвредные расширения для маскировки
	extensions := []struct {
		id   uint16
		data []byte
	}{
		{0xFF01, generateRandomBytes(16)}, // Экспериментальное расширение
		{0x0017, []byte{0x00}},            // Extended Master Secret
		{0x0023, []byte{}},                 // Session Ticket
		{0x0033, generateRandomBytes(8)},  // Key Share
	}
	
	buf := bytes.NewBuffer(data)
	
	// Добавляем случайное количество расширений
	numExtensions := randInt(1, int64(len(extensions)))
	for i := 0; i < int(numExtensions); i++ {
		ext := extensions[i]
		binary.Write(buf, binary.BigEndian, ext.id)
		binary.Write(buf, binary.BigEndian, uint16(len(ext.data)))
		buf.Write(ext.data)
	}
	
	return buf.Bytes()
}

// addTLSPadding добавляет padding к ClientHello
func addTLSPadding(data []byte) []byte {
	// Добавляем padding extension (RFC 7685)
	paddingSize := randInt(100, 300)
	padding := make([]byte, paddingSize)
	
	buf := bytes.NewBuffer(data)
	
	// Padding extension ID: 0x0015
	binary.Write(buf, binary.BigEndian, uint16(0x0015))
	binary.Write(buf, binary.BigEndian, uint16(paddingSize))
	buf.Write(padding)
	
	return buf.Bytes()
}

// shuffleExtensions перемешивает порядок TLS расширений
func shuffleExtensions(data []byte) []byte {
	// Находим начало расширений в ClientHello
	extStart := findExtensionsStart(data)
	if extStart < 0 {
		return data
	}
	
	// Парсим расширения
	extensions := parseExtensions(data[extStart:])
	if len(extensions) <= 1 {
		return data
	}
	
	// Перемешиваем расширения (кроме SNI, который должен быть первым)
	shuffled := make([][]byte, len(extensions))
	copy(shuffled, extensions)
	
	// Находим SNI (extension 0x0000) и оставляем его первым
	sniIndex := -1
	for i, ext := range shuffled {
		if len(ext) >= 2 && ext[0] == 0x00 && ext[1] == 0x00 {
			sniIndex = i
			break
		}
	}
	
	if sniIndex > 0 {
		// Перемещаем SNI в начало
		sni := shuffled[sniIndex]
		copy(shuffled[1:sniIndex+1], shuffled[0:sniIndex])
		shuffled[0] = sni
	}
	
	// Случайно перемешиваем остальные расширения
	for i := 1; i < len(shuffled)-1; i++ {
		j := int(randInt(int64(i), int64(len(shuffled))))
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}
	
	// Собираем обратно
	result := append(data[:extStart], bytes.Join(shuffled, []byte{})...)
	return result
}

// MaskSNI маскирует SNI в ClientHello
func MaskSNI(data []byte, fakeSNI string) []byte {
	sniPos := findSNIPosition(data)
	if sniPos < 0 {
		return data
	}
	
	// Используем техники маскировки SNI
	// 1. Domain fronting - подменяем SNI на другой домен
	// 2. Добавляем нулевые байты
	// 3. Используем IDN (интернационализированные домены)
	
	if fakeSNI != "" {
		return replaceSNI(data, sniPos, fakeSNI)
	}
	
	// Добавляем случайные символы к SNI для обфускации
	return obfuscateSNI(data, sniPos)
}

// replaceSNI заменяет SNI на поддельный
func replaceSNI(data []byte, sniPos int, fakeSNI string) []byte {
	// Находим длину оригинального SNI
	if sniPos+5 >= len(data) {
		return data
	}
	
	sniLen := int(binary.BigEndian.Uint16(data[sniPos+3:sniPos+5]))
	if sniPos+5+sniLen > len(data) {
		return data
	}
	
	// Создаем новый буфер
	buf := bytes.NewBuffer(data[:sniPos+5])
	buf.WriteString(fakeSNI)
	
	// Добавляем остаток данных после SNI
	if sniPos+5+sniLen < len(data) {
		buf.Write(data[sniPos+5+sniLen:])
	}
	
	// Обновляем длину SNI
	result := buf.Bytes()
	binary.BigEndian.PutUint16(result[sniPos+3:sniPos+5], uint16(len(fakeSNI)))
	
	return result
}

// obfuscateSNI обфусцирует SNI добавлением специальных символов
func obfuscateSNI(data []byte, sniPos int) []byte {
	// Техника обфускации: добавляем невидимые Unicode символы
	// или используем punycode для маскировки
	
	if sniPos+5 >= len(data) {
		return data
	}
	
	sniLen := int(binary.BigEndian.Uint16(data[sniPos+3:sniPos+5]))
	if sniPos+5+sniLen > len(data) {
		return data
	}
	
	sni := string(data[sniPos+5 : sniPos+5+sniLen])
	
	// Добавляем zero-width символы для обфускации
	obfuscated := insertZeroWidthChars(sni)
	
	// Создаем новый буфер
	buf := bytes.NewBuffer(data[:sniPos+5])
	buf.WriteString(obfuscated)
	
	// Добавляем остаток данных
	if sniPos+5+sniLen < len(data) {
		buf.Write(data[sniPos+5+sniLen:])
	}
	
	// Обновляем длину
	result := buf.Bytes()
	binary.BigEndian.PutUint16(result[sniPos+3:sniPos+5], uint16(len(obfuscated)))
	
	return result
}

// insertZeroWidthChars вставляет невидимые символы в строку
func insertZeroWidthChars(s string) string {
	// Zero-width символы для обфускации
	zeroWidthChars := []string{
		"\u200B", // Zero-width space
		"\u200C", // Zero-width non-joiner
		"\u200D", // Zero-width joiner
		"\uFEFF", // Zero-width no-break space
	}
	
	result := ""
	for i, ch := range s {
		result += string(ch)
		// Случайно вставляем zero-width символы
		if i < len(s)-1 && randInt(0, 3) == 0 {
			result += zeroWidthChars[randInt(0, int64(len(zeroWidthChars)))]
		}
	}
	
	return result
}

// findExtensionsStart находит начало расширений в ClientHello
func findExtensionsStart(data []byte) int {
	// Упрощенный поиск начала расширений
	// ClientHello структура: 
	// - Handshake header (5 bytes)
	// - Client version (2 bytes)
	// - Random (32 bytes)
	// - Session ID length (1 byte) + Session ID
	// - Cipher suites length (2 bytes) + Cipher suites
	// - Compression methods length (1 byte) + Compression methods
	// - Extensions length (2 bytes) + Extensions
	
	if len(data) < 43 {
		return -1
	}
	
	pos := 43 // Минимальная позиция для расширений
	
	// Пропускаем Session ID
	if pos < len(data) {
		sessionIdLen := int(data[pos])
		pos += 1 + sessionIdLen
	}
	
	// Пропускаем Cipher Suites
	if pos+2 < len(data) {
		cipherSuitesLen := int(binary.BigEndian.Uint16(data[pos:pos+2]))
		pos += 2 + cipherSuitesLen
	}
	
	// Пропускаем Compression Methods
	if pos < len(data) {
		compressionLen := int(data[pos])
		pos += 1 + compressionLen
	}
	
	// Теперь должны быть расширения
	if pos+2 < len(data) {
		return pos + 2 // Пропускаем длину расширений
	}
	
	return -1
}

// parseExtensions парсит TLS расширения
func parseExtensions(data []byte) [][]byte {
	var extensions [][]byte
	pos := 0
	
	for pos+4 < len(data) {
		// Extension type (2 bytes) + length (2 bytes)
		extLen := int(binary.BigEndian.Uint16(data[pos+2:pos+4]))
		totalLen := 4 + extLen
		
		if pos+totalLen <= len(data) {
			extensions = append(extensions, data[pos:pos+totalLen])
			pos += totalLen
		} else {
			break
		}
	}
	
	return extensions
}

// findSNIPosition находит позицию SNI в ClientHello
func findSNIPosition(data []byte) int {
	extStart := findExtensionsStart(data)
	if extStart < 0 {
		return -1
	}
	
	pos := extStart
	for pos+4 < len(data) {
		extType := binary.BigEndian.Uint16(data[pos:pos+2])
		extLen := int(binary.BigEndian.Uint16(data[pos+2:pos+4]))
		
		if extType == 0x0000 { // SNI extension
			return pos
		}
		
		pos += 4 + extLen
	}
	
	return -1
}

// isTLSHandshake проверяет, является ли пакет TLS handshake
func isTLSHandshake(data []byte) bool {
	return len(data) > 5 && data[0] == 0x16
}

// generateRandomBytes генерирует случайные байты
func generateRandomBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

// randomDelay делает случайную задержку
func randomDelay() {
	delay := randInt(1, 50)
	time.Sleep(time.Millisecond * time.Duration(delay))
}

// randInt генерирует случайное число
func randInt(min, max int64) int64 {
	if min >= max {
		return min
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(max-min))
	return n.Int64() + min
}