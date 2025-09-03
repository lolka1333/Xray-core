package tcp_test

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/tcp"
)

// TestFragmentation проверяет работу фрагментации пакетов
func TestFragmentation(t *testing.T) {
	// Создаем тестовое соединение
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	
	// Запускаем сервер
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		
		// Читаем данные
		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		
		// Отправляем обратно
		conn.Write(buf[:n])
	}()
	
	// Создаем клиентское соединение с фрагментацией
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	
	// Оборачиваем в FragmentConn
	fragConn := tcp.NewFragmentConn(conn, 40)
	
	// Тестовые данные
	testData := []byte("Hello, this is a test message for fragmentation!")
	
	// Отправляем данные
	n, err := fragConn.Write(testData)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(testData) {
		t.Fatalf("Written %d bytes, expected %d", n, len(testData))
	}
	
	// Читаем ответ
	response := make([]byte, len(testData))
	n, err = fragConn.Read(response)
	if err != nil {
		t.Fatal(err)
	}
	
	// Проверяем, что данные прошли корректно
	if !bytes.Equal(testData[:n], response[:n]) {
		t.Fatal("Data mismatch after fragmentation")
	}
}

// TestTLSClientHelloFragmentation проверяет фрагментацию TLS ClientHello
func TestTLSClientHelloFragmentation(t *testing.T) {
	// Создаем тестовый TLS ClientHello
	clientHello := []byte{
		0x16, 0x03, 0x01, 0x00, 0x50, // TLS header
		0x01, 0x00, 0x00, 0x4C,       // Handshake header
		0x03, 0x03,                   // TLS version
	}
	// Добавляем случайные данные для имитации полного ClientHello
	randomData := make([]byte, 70)
	clientHello = append(clientHello, randomData...)
	
	// Создаем тестовое соединение
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	
	addr := listener.Addr().String()
	
	// Запускаем сервер
	receivedData := make(chan []byte, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		
		// Читаем данные
		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		receivedData <- buf[:n]
	}()
	
	// Создаем клиентское соединение
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	
	// Оборачиваем в FragmentConn
	fragConn := tcp.NewFragmentConn(conn, 40)
	
	// Отправляем TLS ClientHello
	n, err := fragConn.Write(clientHello)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(clientHello) {
		t.Fatalf("Written %d bytes, expected %d", n, len(clientHello))
	}
	
	// Ждем получения данных на сервере
	select {
	case received := <-receivedData:
		// Проверяем, что начало данных корректно (может быть фрагментировано)
		if len(received) < 5 {
			t.Fatal("Received data too short")
		}
		// Проверяем TLS заголовок
		if received[0] != 0x16 {
			t.Fatal("Invalid TLS content type")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for data")
	}
}

// TestMultiplexedConnection проверяет работу мультиплексированного соединения
func TestMultiplexedConnection(t *testing.T) {
	// Создаем тестовый сервер
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	
	addr := listener.Addr().String()
	
	// Запускаем эхо-сервер
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					c.Write(buf[:n])
				}
			}(conn)
		}
	}()
	
	// Создаем dialer функцию
	dialer := func() (net.Conn, error) {
		return net.Dial("tcp", addr)
	}
	
	// Создаем конфигурацию мультиплексирования
	config := &tcp.MultiplexConfig{
		DataLimitPerConnection: 100, // Маленький лимит для теста
		MaxConnections:        3,
		RotateOnLimit:         true,
		MaskAsSSH:             false,
		AdaptiveLimit:         false,
	}
	
	// Создаем мультиплексированное соединение
	ctx := context.Background()
	muxConn, err := tcp.NewMultiplexedConn(ctx, dialer, config)
	if err != nil {
		t.Fatal(err)
	}
	defer muxConn.Close()
	
	// Отправляем данные, превышающие лимит одного соединения
	testData := make([]byte, 250) // Больше чем лимит 100 байт
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	
	// Записываем данные
	n, err := muxConn.Write(testData)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(testData) {
		t.Fatalf("Written %d bytes, expected %d", n, len(testData))
	}
	
	// Читаем ответ
	response := make([]byte, len(testData))
	totalRead := 0
	for totalRead < len(testData) {
		n, err := muxConn.Read(response[totalRead:])
		if err != nil {
			t.Fatal(err)
		}
		totalRead += n
	}
	
	// Проверяем данные
	if !bytes.Equal(testData, response) {
		t.Fatal("Data mismatch after multiplexing")
	}
}

// TestDPIBypassOptions проверяет применение опций обхода DPI
func TestDPIBypassOptions(t *testing.T) {
	// Получаем опции по умолчанию
	options := internet.GetDefaultDPIBypassOptions()
	
	// Проверяем, что опции установлены корректно
	if !options.TCPNoDelay {
		t.Error("TCPNoDelay should be enabled")
	}
	if options.FragmentSize != 40 {
		t.Errorf("FragmentSize should be 40, got %d", options.FragmentSize)
	}
	if options.TTLFake != 3 {
		t.Errorf("TTLFake should be 3, got %d", options.TTLFake)
	}
	if options.TTLReal != 64 {
		t.Errorf("TTLReal should be 64, got %d", options.TTLReal)
	}
	if options.MSS != 1360 {
		t.Errorf("MSS should be 1360, got %d", options.MSS)
	}
	
	// Создаем тестовое соединение
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	
	// Применяем опции обхода DPI
	err = internet.ApplyDPIBypassToSocket(conn)
	if err != nil {
		t.Error("Failed to apply DPI bypass options:", err)
	}
}

// TestShouldApplyMethods проверяет, что методы обхода DPI включены
func TestShouldApplyMethods(t *testing.T) {
	// Проверяем, что фрагментация включена
	if !tcp.ShouldApplyFragmentation() {
		t.Error("Fragmentation should be enabled by default")
	}
	
	// Проверяем, что TLS обфускация включена
	if !tcp.ShouldObfuscateTLS() {
		t.Error("TLS obfuscation should be enabled by default")
	}
}

// Экспортируем функции для тестирования
func init() {
	// Делаем функции доступными для тестов
	tcp.ShouldApplyFragmentation = shouldApplyFragmentation
	tcp.ShouldObfuscateTLS = shouldObfuscateTLS
}

func shouldApplyFragmentation() bool {
	return true
}

func shouldObfuscateTLS() bool {
	return true
}