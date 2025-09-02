package internet

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strings"
	"time"
)

// HTTPObfuscator обфускатор HTTP трафика для обхода DPI
type HTTPObfuscator struct {
	conn              net.Conn
	fragmentHeaders   bool
	randomizeCase     bool
	insertSpaces      bool
	useChunkedEncoding bool
}

// NewHTTPObfuscator создает новый HTTP обфускатор
func NewHTTPObfuscator(conn net.Conn) *HTTPObfuscator {
	return &HTTPObfuscator{
		conn:              conn,
		fragmentHeaders:   true,
		randomizeCase:     true,
		insertSpaces:      true,
		useChunkedEncoding: true,
	}
}

// ObfuscateRequest обфусцирует HTTP запрос
func (ho *HTTPObfuscator) ObfuscateRequest(request []byte) ([]byte, error) {
	// Парсим HTTP запрос
	lines := bytes.Split(request, []byte("\r\n"))
	if len(lines) < 1 {
		return request, nil
	}
	
	// Обфусцируем первую строку (метод, путь, версия)
	lines[0] = ho.obfuscateRequestLine(lines[0])
	
	// Обфусцируем заголовки
	for i := 1; i < len(lines); i++ {
		if len(lines[i]) == 0 {
			break // Конец заголовков
		}
		lines[i] = ho.obfuscateHeader(lines[i])
	}
	
	// Добавляем дополнительные заголовки для маскировки
	lines = ho.addDecoyHeaders(lines)
	
	// Собираем обратно
	result := bytes.Join(lines, []byte("\r\n"))
	
	// Применяем фрагментацию заголовков
	if ho.fragmentHeaders {
		return ho.fragmentHTTPHeaders(result), nil
	}
	
	return result, nil
}

// obfuscateRequestLine обфусцирует первую строку HTTP запроса
func (ho *HTTPObfuscator) obfuscateRequestLine(line []byte) []byte {
	parts := bytes.Split(line, []byte(" "))
	if len(parts) != 3 {
		return line
	}
	
	method := parts[0]
	path := parts[1]
	version := parts[2]
	
	// Рандомизируем регистр метода
	if ho.randomizeCase {
		method = ho.randomizeMethodCase(method)
	}
	
	// Обфусцируем путь
	path = ho.obfuscatePath(path)
	
	// Добавляем пробелы
	if ho.insertSpaces {
		// Добавляем дополнительные пробелы между частями
		spaces := strings.Repeat(" ", rand.Intn(3)+1)
		return bytes.Join([][]byte{method, path, version}, []byte(spaces))
	}
	
	return bytes.Join([][]byte{method, path, version}, []byte(" "))
}

// randomizeMethodCase рандомизирует регистр HTTP метода
func (ho *HTTPObfuscator) randomizeMethodCase(method []byte) []byte {
	// Некоторые DPI чувствительны к регистру
	// GET -> GeT, Get, gET и т.д.
	result := make([]byte, len(method))
	for i, b := range method {
		if rand.Intn(2) == 0 {
			result[i] = bytes.ToLower([]byte{b})[0]
		} else {
			result[i] = bytes.ToUpper([]byte{b})[0]
		}
	}
	return result
}

// obfuscatePath обфусцирует URL путь
func (ho *HTTPObfuscator) obfuscatePath(path []byte) []byte {
	// Добавляем URL encoding для обычных символов
	// Добавляем двойные слеши
	// Используем альтернативные представления
	
	pathStr := string(path)
	
	// Случайно добавляем двойные слеши
	if rand.Intn(3) == 0 {
		pathStr = strings.ReplaceAll(pathStr, "/", "//")
	}
	
	// Добавляем фрагмент (игнорируется сервером)
	if !strings.Contains(pathStr, "#") && rand.Intn(2) == 0 {
		pathStr += "#" + ho.generateRandomString(8)
	}
	
	// Добавляем случайные параметры
	if !strings.Contains(pathStr, "?") && rand.Intn(3) == 0 {
		pathStr += "?" + ho.generateRandomParam()
	}
	
	return []byte(pathStr)
}

// obfuscateHeader обфусцирует HTTP заголовок
func (ho *HTTPObfuscator) obfuscateHeader(header []byte) []byte {
	parts := bytes.SplitN(header, []byte(":"), 2)
	if len(parts) != 2 {
		return header
	}
	
	name := parts[0]
	value := parts[1]
	
	// Рандомизируем регистр имени заголовка
	if ho.randomizeCase {
		name = ho.randomizeHeaderCase(name)
	}
	
	// Добавляем пробелы
	if ho.insertSpaces {
		// Добавляем пробелы после двоеточия
		spaces := strings.Repeat(" ", rand.Intn(3)+1)
		return bytes.Join([][]byte{name, value}, []byte(":" + spaces))
	}
	
	// Специальная обработка для Host заголовка
	if bytes.EqualFold(name, []byte("Host")) {
		value = ho.obfuscateHost(value)
	}
	
	return bytes.Join([][]byte{name, value}, []byte(":"))
}

// randomizeHeaderCase рандомизирует регистр заголовка
func (ho *HTTPObfuscator) randomizeHeaderCase(name []byte) []byte {
	// Host -> HoSt, host, HOST и т.д.
	result := make([]byte, len(name))
	for i, b := range name {
		if b == '-' {
			result[i] = b
			continue
		}
		if rand.Intn(2) == 0 {
			result[i] = bytes.ToLower([]byte{b})[0]
		} else {
			result[i] = bytes.ToUpper([]byte{b})[0]
		}
	}
	return result
}

// obfuscateHost обфусцирует Host заголовок
func (ho *HTTPObfuscator) obfuscateHost(host []byte) []byte {
	hostStr := strings.TrimSpace(string(host))
	
	// Добавляем порт если его нет
	if !strings.Contains(hostStr, ":") && rand.Intn(3) == 0 {
		if strings.HasPrefix(hostStr, "https") {
			hostStr += ":443"
		} else {
			hostStr += ":80"
		}
	}
	
	// Добавляем точку в конце домена (FQDN)
	if !strings.HasSuffix(hostStr, ".") && rand.Intn(2) == 0 {
		if colonIndex := strings.Index(hostStr, ":"); colonIndex > 0 {
			// Добавляем точку перед портом
			hostStr = hostStr[:colonIndex] + "." + hostStr[colonIndex:]
		} else {
			hostStr += "."
		}
	}
	
	return []byte(" " + hostStr)
}

// addDecoyHeaders добавляет ложные заголовки для маскировки
func (ho *HTTPObfuscator) addDecoyHeaders(lines [][]byte) [][]byte {
	decoyHeaders := [][]byte{
		[]byte("X-Forwarded-For: " + ho.generateRandomIP()),
		[]byte("X-Real-IP: " + ho.generateRandomIP()),
		[]byte("X-Originating-IP: " + ho.generateRandomIP()),
		[]byte("X-Remote-IP: " + ho.generateRandomIP()),
		[]byte("X-Client-IP: " + ho.generateRandomIP()),
		[]byte("X-Forwarded-Proto: https"),
		[]byte("X-Requested-With: XMLHttpRequest"),
		[]byte("Cache-Control: no-cache"),
		[]byte("Pragma: no-cache"),
		[]byte("DNT: 1"),
		[]byte("Upgrade-Insecure-Requests: 1"),
		[]byte("X-Custom-Header: " + ho.generateRandomString(16)),
	}
	
	// Находим конец заголовков
	endIndex := -1
	for i, line := range lines {
		if len(line) == 0 {
			endIndex = i
			break
		}
	}
	
	if endIndex < 0 {
		endIndex = len(lines)
	}
	
	// Добавляем случайное количество ложных заголовков
	numDecoys := rand.Intn(4) + 1
	for i := 0; i < numDecoys && i < len(decoyHeaders); i++ {
		// Вставляем в случайную позицию среди заголовков
		insertPos := rand.Intn(endIndex-1) + 1
		lines = append(lines[:insertPos], append([][]byte{decoyHeaders[i]}, lines[insertPos:]...)...)
		endIndex++
	}
	
	return lines
}

// fragmentHTTPHeaders фрагментирует HTTP заголовки
func (ho *HTTPObfuscator) fragmentHTTPHeaders(data []byte) []byte {
	// Разбиваем заголовки на мелкие части для обхода DPI
	// DPI часто ищет паттерны в первых пакетах
	
	if len(data) < 10 {
		return data
	}
	
	// Находим конец первой строки
	firstLineEnd := bytes.Index(data, []byte("\r\n"))
	if firstLineEnd < 0 {
		return data
	}
	
	// Фрагментируем первую строку
	fragments := [][]byte{}
	
	// Разбиваем первую строку на части
	firstLine := data[:firstLineEnd+2]
	
	// Отправляем метод отдельно
	if spaceIndex := bytes.Index(firstLine, []byte(" ")); spaceIndex > 0 {
		fragments = append(fragments, firstLine[:spaceIndex+1])
		fragments = append(fragments, firstLine[spaceIndex+1:])
	} else {
		fragments = append(fragments, firstLine)
	}
	
	// Добавляем остальные заголовки маленькими порциями
	remaining := data[firstLineEnd+2:]
	chunkSize := 20 + rand.Intn(30) // Случайный размер фрагмента
	
	for len(remaining) > 0 {
		if len(remaining) <= chunkSize {
			fragments = append(fragments, remaining)
			break
		}
		
		// Стараемся не разрывать заголовки посередине
		chunk := remaining[:chunkSize]
		if crlfIndex := bytes.LastIndex(chunk, []byte("\r\n")); crlfIndex > 0 {
			chunk = remaining[:crlfIndex+2]
		}
		
		fragments = append(fragments, chunk)
		remaining = remaining[len(chunk):]
	}
	
	// Записываем фрагменты с задержками
	return ho.writeFragments(fragments)
}

// writeFragments записывает фрагменты с задержками
func (ho *HTTPObfuscator) writeFragments(fragments [][]byte) []byte {
	result := bytes.NewBuffer(nil)
	
	for i, fragment := range fragments {
		result.Write(fragment)
		
		// Добавляем маркер задержки (будет обработан при отправке)
		if i < len(fragments)-1 {
			// Специальный маркер для задержки
			result.Write([]byte{0xFF, 0xFE})
		}
	}
	
	return result.Bytes()
}

// generateRandomIP генерирует случайный IP адрес
func (ho *HTTPObfuscator) generateRandomIP() string {
	return fmt.Sprintf("%d.%d.%d.%d",
		rand.Intn(256), rand.Intn(256),
		rand.Intn(256), rand.Intn(256))
}

// generateRandomString генерирует случайную строку
func (ho *HTTPObfuscator) generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

// generateRandomParam генерирует случайный URL параметр
func (ho *HTTPObfuscator) generateRandomParam() string {
	params := []string{
		"utm_source=" + ho.generateRandomString(8),
		"utm_medium=" + ho.generateRandomString(6),
		"ref=" + ho.generateRandomString(10),
		"_=" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"cb=" + fmt.Sprintf("%d", rand.Int63()),
		"rnd=" + ho.generateRandomString(12),
	}
	
	numParams := rand.Intn(3) + 1
	selected := make([]string, 0, numParams)
	
	for i := 0; i < numParams && i < len(params); i++ {
		selected = append(selected, params[rand.Intn(len(params))])
	}
	
	return strings.Join(selected, "&")
}

// HTTPObfuscatedConn обертка для соединения с HTTP обфускацией
type HTTPObfuscatedConn struct {
	net.Conn
	obfuscator *HTTPObfuscator
	isHTTP     bool
}

// NewHTTPObfuscatedConn создает новое соединение с HTTP обфускацией
func NewHTTPObfuscatedConn(conn net.Conn) *HTTPObfuscatedConn {
	return &HTTPObfuscatedConn{
		Conn:       conn,
		obfuscator: NewHTTPObfuscator(conn),
		isHTTP:     false,
	}
}

// Write отправляет данные с обфускацией
func (hoc *HTTPObfuscatedConn) Write(b []byte) (int, error) {
	// Проверяем, является ли это HTTP запросом
	if !hoc.isHTTP && isHTTPRequest(b) {
		hoc.isHTTP = true
		obfuscated, err := hoc.obfuscator.ObfuscateRequest(b)
		if err != nil {
			return 0, err
		}
		
		// Отправляем с обработкой маркеров задержки
		return hoc.writeWithDelays(obfuscated)
	}
	
	// Обычная отправка для не-HTTP данных
	return hoc.Conn.Write(b)
}

// writeWithDelays отправляет данные с обработкой маркеров задержки
func (hoc *HTTPObfuscatedConn) writeWithDelays(data []byte) (int, error) {
	totalWritten := 0
	buffer := bytes.NewBuffer(data)
	
	for buffer.Len() > 0 {
		// Ищем маркер задержки
		chunk := buffer.Bytes()
		if markerIndex := bytes.Index(chunk, []byte{0xFF, 0xFE}); markerIndex >= 0 {
			// Отправляем данные до маркера
			if markerIndex > 0 {
				n, err := hoc.Conn.Write(chunk[:markerIndex])
				if err != nil {
					return totalWritten, err
				}
				totalWritten += n
			}
			
			// Пропускаем маркер
			buffer.Next(markerIndex + 2)
			
			// Делаем задержку
			time.Sleep(time.Millisecond * time.Duration(rand.Intn(50)+10))
		} else {
			// Отправляем оставшиеся данные
			n, err := hoc.Conn.Write(chunk)
			if err != nil {
				return totalWritten, err
			}
			totalWritten += n
			break
		}
	}
	
	return len(data) - bytes.Count(data, []byte{0xFF, 0xFE})*2, nil
}

// isHTTPRequest проверяет, является ли пакет HTTP запросом
func isHTTPRequest(data []byte) bool {
	// Проверяем наличие HTTP методов
	methods := []string{"GET ", "POST ", "PUT ", "DELETE ", "HEAD ", "OPTIONS ", "CONNECT ", "TRACE ", "PATCH "}
	
	dataStr := string(data[:min(10, len(data))])
	dataUpper := strings.ToUpper(dataStr)
	
	for _, method := range methods {
		if strings.HasPrefix(dataUpper, method) {
			return true
		}
	}
	
	return false
}



// HTTPFragmentWriter писатель с фрагментацией для HTTP
type HTTPFragmentWriter struct {
	writer io.Writer
}

// NewHTTPFragmentWriter создает новый писатель с фрагментацией
func NewHTTPFragmentWriter(w io.Writer) *HTTPFragmentWriter {
	return &HTTPFragmentWriter{writer: w}
}

// Write записывает данные с фрагментацией
func (hfw *HTTPFragmentWriter) Write(p []byte) (n int, err error) {
	// Фрагментируем HTTP заголовки на мелкие части
	if isHTTPRequest(p) {
		// Разбиваем на фрагменты по 1-40 байт
		fragmentSize := 1 + rand.Intn(40)
		totalWritten := 0
		
		for i := 0; i < len(p); i += fragmentSize {
			end := i + fragmentSize
			if end > len(p) {
				end = len(p)
			}
			
			chunk := p[i:end]
			written, err := hfw.writer.Write(chunk)
			if err != nil {
				return totalWritten, err
			}
			totalWritten += written
			
			// Задержка между фрагментами
			if end < len(p) {
				time.Sleep(time.Microsecond * time.Duration(rand.Intn(1000)))
			}
		}
		
		return totalWritten, nil
	}
	
	// Обычная запись для не-HTTP данных
	return hfw.writer.Write(p)
}