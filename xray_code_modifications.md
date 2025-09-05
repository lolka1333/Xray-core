# Минимальные модификации кода Xray для обхода блокировок в мобильных сетях

## 1. Модификация размера первых пакетов (Packet Padding)

### Проблема
DPI системы определяют Xray по характерному размеру первых пакетов при установлении соединения.

### Решение - добавление случайного паддинга

**Файл:** `transport/internet/tcp/sockopt_linux.go`

```go
// Добавить в функцию applyOutboundSocketOptions
func applyOutboundSocketOptions(network string, address string, fd uintptr, config *SocketConfig) error {
    // Существующий код...
    
    // Добавить случайный паддинг к первым пакетам
    if config.TcpNoDelay {
        // Добавляем случайную задержку 1-50ms для первых пакетов
        randomDelay := time.Duration(rand.Intn(50)+1) * time.Millisecond
        time.Sleep(randomDelay)
    }
    
    // Изменяем размер TCP окна случайным образом
    windowSize := 65536 + rand.Intn(32768)
    syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_WINDOW_CLAMP, windowSize)
    
    return nil
}
```

## 2. Модификация TLS ClientHello

### Проблема
Стандартный TLS fingerprint Xray легко определяется DPI.

### Решение - рандомизация TLS расширений

**Файл:** `transport/internet/tls/config.go`

```go
// Модифицировать функцию GetTLSConfig
func (c *Config) GetTLSConfig(opts ...Option) *tls.Config {
    config := &tls.Config{
        ClientSessionCache: globalSessionCache,
        InsecureSkipVerify: c.AllowInsecure,
        NextProtos:         c.NextProtocol,
        SessionTicketsDisabled: c.DisableSessionResumption,
    }
    
    // Добавляем рандомизацию расширений
    if len(config.NextProtos) > 0 {
        // Перемешиваем порядок ALPN протоколов случайным образом
        rand.Shuffle(len(config.NextProtos), func(i, j int) {
            config.NextProtos[i], config.NextProtos[j] = config.NextProtos[j], config.NextProtos[i]
        })
    }
    
    // Добавляем случайные расширения
    config.CurvePreferences = []tls.CurveID{
        tls.X25519,
        tls.CurveP256,
        tls.CurveP384,
    }
    // Рандомизируем порядок кривых
    rand.Shuffle(len(config.CurvePreferences), func(i, j int) {
        config.CurvePreferences[i], config.CurvePreferences[j] = 
            config.CurvePreferences[j], config.CurvePreferences[i]
    })
    
    return config
}
```

## 3. Изменение временных паттернов (Timing Obfuscation)

### Проблема
DPI анализирует временные интервалы между пакетами.

### Решение - добавление случайных задержек

**Файл:** `proxy/vless/outbound/outbound.go`

```go
// В функции Process добавить случайные микрозадержки
func (h *Handler) Process(ctx context.Context, link *transport.Link, dialer internet.Dialer) error {
    // После установления соединения
    conn, err := dialer.Dial(ctx, dest)
    if err != nil {
        return newError("failed to connect to ", dest).Base(err)
    }
    
    // Добавляем случайную начальную задержку 10-100ms
    initialDelay := time.Duration(rand.Intn(90)+10) * time.Millisecond
    time.Sleep(initialDelay)
    
    // При отправке данных добавляем микрозадержки
    go func() {
        for {
            // Случайная задержка 0-5ms между чанками данных
            microDelay := time.Duration(rand.Intn(6)) * time.Millisecond
            time.Sleep(microDelay)
            // Продолжаем передачу...
        }
    }()
    
    // Остальной код...
}
```

## 4. Фрагментация первого пакета

### Проблема
DPI анализирует структуру первого пакета целиком.

### Решение - разбивка первого пакета на части

**Файл:** `common/protocol/headers.go`

```go
// Добавить функцию фрагментации
func FragmentFirstPacket(data []byte) [][]byte {
    if len(data) <= 100 {
        return [][]byte{data}
    }
    
    // Разбиваем первый пакет на 2-3 случайные части
    numFragments := rand.Intn(2) + 2
    fragments := make([][]byte, numFragments)
    
    fragmentSize := len(data) / numFragments
    for i := 0; i < numFragments-1; i++ {
        // Добавляем случайную вариацию размера ±20%
        variation := rand.Intn(fragmentSize/5*2) - fragmentSize/5
        actualSize := fragmentSize + variation
        if actualSize > len(data)-fragmentSize {
            actualSize = len(data) - fragmentSize
        }
        fragments[i] = data[:actualSize]
        data = data[actualSize:]
    }
    fragments[numFragments-1] = data
    
    return fragments
}

// Использовать в функции отправки
func (w *Writer) WriteHeader(header interface{}) error {
    // Сериализуем заголовок
    headerBytes := serializeHeader(header)
    
    // Фрагментируем первый пакет
    fragments := FragmentFirstPacket(headerBytes)
    
    // Отправляем фрагменты с микрозадержками
    for i, fragment := range fragments {
        if i > 0 {
            time.Sleep(time.Duration(rand.Intn(10)+1) * time.Millisecond)
        }
        if _, err := w.writer.Write(fragment); err != nil {
            return err
        }
    }
    
    return nil
}
```

## 5. Изменение идентификаторов протокола

### Проблема
VMess и VLESS имеют узнаваемые магические байты.

### Решение - XOR обфускация первых байтов

**Файл:** `proxy/vmess/encoding/client.go`

```go
// Добавить простую XOR обфускацию
func ObfuscateHeader(data []byte, key byte) {
    // XOR первые 16 байт с ключом, производным от времени
    key = byte(time.Now().Unix() % 256)
    for i := 0; i < 16 && i < len(data); i++ {
        data[i] ^= key
        key = (key + 7) % 256 // Простая PRNG
    }
}

// В функции EncodeRequestHeader
func (c *ClientSession) EncodeRequestHeader(header *protocol.RequestHeader, writer io.Writer) error {
    // Существующий код подготовки заголовка...
    
    // Обфусцируем первые байты перед отправкой
    ObfuscateHeader(buffer.Bytes(), 0)
    
    // Отправляем обфусцированные данные
    _, err := writer.Write(buffer.Bytes())
    return err
}
```

## 6. Простейший патч для быстрого тестирования

### Минимальное изменение в конфигурации (без перекомпиляции)

**config.json на клиенте:**

```json
{
  "outbounds": [{
    "protocol": "vless",
    "settings": {
      "vnext": [{
        "address": "your-server.com",
        "port": 443,
        "users": [{
          "id": "your-uuid",
          "encryption": "none",
          "flow": "xtls-rprx-vision"
        }]
      }]
    },
    "streamSettings": {
      "network": "tcp",
      "security": "tls",
      "tlsSettings": {
        "serverName": "microsoft.com",  // Маскируемся под Microsoft
        "allowInsecure": false,
        "fingerprint": "chrome",  // ВАЖНО: используем fingerprint
        "alpn": ["h2", "http/1.1"]  // Меняем порядок ALPN
      },
      "sockopt": {
        "tcpFastOpen": true,  // Включаем TCP Fast Open
        "tcpNoDelay": true,
        "mark": 255,  // Добавляем метку для обхода
        "tcpKeepAliveInterval": 30,
        "dialerProxy": "fragment"  // Включаем фрагментацию
      }
    },
    "mux": {
      "enabled": true,
      "concurrency": 8,  // Мультиплексирование соединений
      "xudpConcurrency": 16
    }
  }]
}
```

## 7. Скрипт для автоматической модификации Xray

```bash
#!/bin/bash
# patch_xray.sh - Простой патч для Xray

# Скачиваем исходники
git clone https://github.com/XTLS/Xray-core.git
cd Xray-core

# Применяем патч для добавления случайных задержек
cat > timing_patch.go << 'EOF'
//go:build linux
// +build linux

package tcp

import (
    "math/rand"
    "time"
)

func init() {
    // Добавляем случайную задержку при инициализации
    rand.Seed(time.Now().UnixNano())
    time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
}
EOF

# Копируем патч в нужную директорию
cp timing_patch.go transport/internet/tcp/

# Компилируем с модификациями
CGO_ENABLED=0 go build -o xray -trimpath -ldflags "-s -w" ./main

echo "Патченная версия Xray готова!"
```

## Рекомендации по применению

1. **Начните с конфигурационных изменений** - они не требуют перекомпиляции
2. **Используйте фрагментацию** - добавьте `"dialerProxy": "fragment"` в sockopt
3. **Обязательно включите uTLS** - `"fingerprint": "chrome"` или `"firefox"`
4. **Используйте мультиплексирование** - включите mux для смешивания трафика
5. **Меняйте порты** - используйте 8443, 2053, 2083 вместо 443

## Важные замечания

- Эти модификации могут нарушить совместимость с официальными версиями
- Тестируйте изменения постепенно
- Некоторые модификации могут снизить производительность
- Регулярно обновляйте патчи при выходе новых версий Xray

## Альтернатива - использование готовых форков

Существуют модифицированные версии Xray с улучшенной устойчивостью к DPI:

1. **Xray-core с патчами от иранского сообщества**
2. **V2Ray-core с модификациями для обхода GFW**
3. **Custom builds с дополнительной обфускацией**

Эти форки уже содержат многие из описанных модификаций.