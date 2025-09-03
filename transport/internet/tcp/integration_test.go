package tcp_test

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/xtls/xray-core/transport/internet/tcp"
)

// TestDPIBypassIntegration проверяет интеграцию всех методов обхода DPI
func TestDPIBypassIntegration(t *testing.T) {
	// Создаем сервер
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	
	// Буфер для получения данных
	receivedData := make(chan []byte, 1)
	
	// Запускаем сервер
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		
		// Читаем все данные
		buf := make([]byte, 4096)
		totalBuf := make([]byte, 0)
		for {
			conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			n, err := conn.Read(buf)
			if n > 0 {
				totalBuf = append(totalBuf, buf[:n]...)
			}
			if err != nil {
				break
			}
		}
		
		if len(totalBuf) > 0 {
			receivedData <- totalBuf
			// Отправляем эхо
			conn.Write(totalBuf)
		}
	}()
	
	// Даем серверу время запуститься
	time.Sleep(10 * time.Millisecond)
	
	// Создаем клиентское соединение
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	
	// Проверяем фрагментацию
	t.Run("Fragmentation", func(t *testing.T) {
		// Оборачиваем в FragmentConn
		fragConn := tcp.NewFragmentConn(conn, 40)
		
		// Тестовые данные
		testData := []byte("Test DPI bypass with fragmentation")
		
		// Отправляем
		n, err := fragConn.Write(testData)
		if err != nil {
			t.Fatal(err)
		}
		if n != len(testData) {
			t.Fatalf("Written %d bytes, expected %d", n, len(testData))
		}
		
		// Небольшая задержка для получения данных
		time.Sleep(100 * time.Millisecond)
		
		select {
		case received := <-receivedData:
			if !bytes.Contains(received, testData) {
				t.Fatal("Data not received correctly through fragmentation")
			}
			t.Logf("Successfully received %d bytes through fragmentation", len(received))
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for fragmented data")
		}
	})
}

// TestDPIBypassEnabled проверяет, что все методы обхода включены
func TestDPIBypassEnabled(t *testing.T) {
	// Проверяем, что фрагментация включена
	if !tcp.ShouldApplyFragmentation() {
		t.Error("TCP fragmentation should be enabled")
	}
	
	// Проверяем, что TLS обфускация включена
	if !tcp.ShouldObfuscateTLS() {
		t.Error("TLS obfuscation should be enabled")
	}
	
	t.Log("All DPI bypass methods are enabled")
}

// BenchmarkFragmentation бенчмарк фрагментации
func BenchmarkFragmentation(b *testing.B) {
	// Создаем пару соединений через pipe
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	
	// Эхо-сервер
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := server.Read(buf)
			if err != nil {
				return
			}
			server.Write(buf[:n])
		}
	}()
	
	// Оборачиваем клиента в FragmentConn
	fragConn := tcp.NewFragmentConn(client, 40)
	
	// Тестовые данные
	testData := make([]byte, 1024)
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	
	// Бенчмарк
	b.ResetTimer()
	b.SetBytes(int64(len(testData)))
	
	for i := 0; i < b.N; i++ {
		_, err := fragConn.Write(testData)
		if err != nil {
			b.Fatal(err)
		}
		
		// Читаем ответ
		response := make([]byte, len(testData))
		totalRead := 0
		for totalRead < len(testData) {
			n, err := fragConn.Read(response[totalRead:])
			if err != nil {
				b.Fatal(err)
			}
			totalRead += n
		}
	}
}