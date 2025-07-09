# Реализация обхода российского DPI в протоколе xhttp

## Краткое резюме

Данный документ представляет конкретную реализацию улучшений протокола xhttp для обхода российского DPI (Deep Packet Inspection) без создания нового протокола. Все изменения интегрированы в существующий код xray-core.

## Анализ проблемы

### Российские методы блокировки (2024-2025)

1. **Лимит 15-20KB** на TLS 1.3 соединения к зарубежным серверам
2. **Энтропийная классификация** высокоэнтропийного трафика
3. **Анализ паттернов соединений** (продолжительность, распределение пакетов)
4. **Блокировка популярных VPN-серверов** (Hetzner, DigitalOcean, AWS)

## Реализованные изменения

### 1. Расширение конфигурации протокола

**Файл: `transport/internet/splithttp/config.proto`**

Добавлены новые структуры конфигурации:

```protobuf
message DPIBypassConfig {
  bool enabled = 1;
  ResponseFragmentationConfig responseFragmentation = 2;
  ConnectionManagementConfig connectionManagement = 3;
  TrafficMaskingConfig trafficMasking = 4;
  CDNHoppingConfig cdnHopping = 5;
  EntropyReductionConfig entropyReduction = 6;
}

message ResponseFragmentationConfig {
  bool enabled = 1;
  int32 maxChunkSize = 2;
  RangeConfig randomDelay = 3;
  bool connectionPooling = 4;
}

message ConnectionManagementConfig {
  int32 maxConnectionLifetime = 1;
  bool connectionRotation = 2;
  int32 parallelConnections = 3;
  string loadBalancing = 4;
}

message TrafficMaskingConfig {
  bool userAgentRotation = 1;
  repeated string acceptHeaders = 2;
  bool refererGeneration = 3;
  bool cookieManagement = 4;
  repeated string compressionSupport = 5;
  bool browserBehaviorSimulation = 6;
}

message CDNHoppingConfig {
  bool enabled = 1;
  repeated string providers = 2;
  int32 rotationInterval = 3;
  int32 failoverThreshold = 4;
}

message EntropyReductionConfig {
  bool enabled = 1;
  string method = 2;
  double targetEntropy = 3;
  bool patternInjection = 4;
}
```

### 2. Новый модуль обхода DPI

**Файл: `transport/internet/splithttp/dpi_bypass.go`**

Реализованы следующие классы:

#### ResponseFragmenter
- Автоматическое разбиение ответов на фрагменты <15KB
- Случайные задержки между фрагментами
- Управление пулом соединений

#### ConnectionManager
- Ротация соединений с максимальным временем жизни
- Балансировка нагрузки (round-robin, random)
- Управление параллельными соединениями

#### TrafficMasker
- Ротация User-Agent
- Генерация реалистичных заголовков
- Управление cookies
- Симуляция поведения браузера

#### CDNHopper
- Переключение между CDN-провайдерами
- Автоматический failover
- Периодическая ротация

#### EntropyReducer
- Снижение энтропии трафика
- Вставка структурированных паттернов
- Целевая энтропия 0.7

### 3. Интеграция в клиентскую часть

**Файл: `transport/internet/splithttp/client.go`**

Добавлено поле `dpiBypassManager` в структуру `DefaultDialerClient`:

```go
type DefaultDialerClient struct {
    transportConfig *Config
    client          *http.Client
    closed          bool
    httpVersion     string
    uploadRawPool    *sync.Pool
    dialUploadConn   func(ctxInner context.Context) (net.Conn, error)
    dpiBypassManager *DPIBypassManager
}
```

Обработка запросов с применением обхода DPI в методах `OpenStream` и `PostPacket`.

### 4. Интеграция в серверную часть

**Файл: `transport/internet/splithttp/hub.go`**

Добавлено поле `dpiBypassManager` в структуру `requestHandler` и модификация `httpServerConn` для обработки ответов с фрагментацией.

### 5. Примеры конфигурации

**Файл: `transport/internet/splithttp/config_examples.json`**

Создано 4 примера конфигурации:

1. **basic_dpi_bypass** - базовая конфигурация
2. **advanced_dpi_bypass** - расширенная конфигурация с CDN hopping
3. **minimal_dpi_bypass** - минимальная конфигурация
4. **russian_specific_config** - специфичная для России конфигурация

## Основные функции

### 1. Фрагментация ответов

```go
func (rf *ResponseFragmenter) FragmentResponse(ctx context.Context, data []byte, writer http.ResponseWriter) error {
    if !rf.config.Enabled || len(data) <= int(rf.config.MaxChunkSize) {
        _, err := writer.Write(data)
        return err
    }

    chunkSize := int(rf.config.MaxChunkSize)
    chunks := make([][]byte, 0, (len(data)+chunkSize-1)/chunkSize)

    for i := 0; i < len(data); i += chunkSize {
        end := i + chunkSize
        if end > len(data) {
            end = len(data)
        }
        chunks = append(chunks, data[i:end])
    }

    // Отправляем первый фрагмент немедленно
    if len(chunks) > 0 {
        if _, err := writer.Write(chunks[0]); err != nil {
            return err
        }
        writer.(http.Flusher).Flush()
    }

    // Отправляем остальные фрагменты с задержками
    for i := 1; i < len(chunks); i++ {
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
```

### 2. Маскировка трафика

```go
func (tm *TrafficMasker) MaskRequest(req *http.Request) {
    if tm.config.UserAgentRotation {
        req.Header.Set("User-Agent", tm.getRandomUserAgent())
    }

    if tm.config.RefererGeneration {
        referer := tm.generateReferer(req.URL.Host)
        if referer != "" {
            req.Header.Set("Referer", referer)
        }
    }

    // Устанавливаем обычные заголовки браузера
    req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.8")
    req.Header.Set("Accept-Encoding", strings.Join(tm.config.CompressionSupport, ", "))
    req.Header.Set("Connection", "keep-alive")
    req.Header.Set("Upgrade-Insecure-Requests", "1")
    req.Header.Set("Sec-Fetch-Dest", "document")
    req.Header.Set("Sec-Fetch-Mode", "navigate")
    req.Header.Set("Sec-Fetch-Site", "none")
    
    if tm.config.CookieManagement {
        tm.addCookies(req)
    }
}
```

### 3. Снижение энтропии

```go
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
```

## Использование

### Конфигурация клиента

```json
{
  "network": "tcp",
  "security": "tls",
  "tlsSettings": {
    "serverName": "example.com",
    "allowInsecure": false,
    "fingerprint": "chrome"
  },
  "xhttpSettings": {
    "path": "/api/v1/",
    "host": "example.com",
    "mode": "packet-up",
    "dpiBypass": {
      "enabled": true,
      "responseFragmentation": {
        "enabled": true,
        "maxChunkSize": 14000,
        "randomDelay": {
          "from": 100,
          "to": 500
        },
        "connectionPooling": true
      },
      "connectionManagement": {
        "maxConnectionLifetime": 30,
        "connectionRotation": true,
        "parallelConnections": 3,
        "loadBalancing": "round-robin"
      },
      "trafficMasking": {
        "userAgentRotation": true,
        "acceptHeaders": ["text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"],
        "refererGeneration": true,
        "cookieManagement": true,
        "compressionSupport": ["gzip", "deflate", "br"],
        "browserBehaviorSimulation": true
      },
      "entropyReduction": {
        "enabled": true,
        "method": "structured-padding",
        "targetEntropy": 0.7,
        "patternInjection": true
      }
    }
  }
}
```

### Конфигурация сервера

```json
{
  "network": "tcp",
  "security": "tls",
  "tlsSettings": {
    "certificates": [
      {
        "certificateFile": "/path/to/cert.pem",
        "keyFile": "/path/to/key.pem"
      }
    ]
  },
  "xhttpSettings": {
    "path": "/api/v1/",
    "host": "example.com",
    "dpiBypass": {
      "enabled": true,
      "responseFragmentation": {
        "enabled": true,
        "maxChunkSize": 14000,
        "randomDelay": {
          "from": 100,
          "to": 500
        }
      },
      "trafficMasking": {
        "browserBehaviorSimulation": true
      }
    }
  }
}
```

## Принцип работы

1. **Запрос клиента:**
   - Применяется маскировка трафика (User-Agent, headers, cookies)
   - Используется ротация соединений
   - Добавляется структурированное заполнение для снижения энтропии

2. **Ответ сервера:**
   - Автоматическая фрагментация ответов >15KB
   - Случайные задержки между фрагментами
   - Снижение энтропии данных

3. **Управление соединениями:**
   - Ротация соединений каждые 30 секунд
   - Балансировка нагрузки между несколькими соединениями
   - Переключение между CDN-провайдерами при сбоях

## Преимущества реализации

1. **Без изменения протокола** - все изменения в рамках существующего xhttp
2. **Обратная совместимость** - старые конфигурации продолжают работать
3. **Гибкость** - каждая функция может быть отключена отдельно
4. **Эффективность** - минимальные накладные расходы при отключенном обходе DPI
5. **Специфичность** - оптимизировано для российских условий блокировки

## Тестирование

Для тестирования функций обхода DPI:

1. Включите `dpiBypass.enabled = true` в конфигурации
2. Настройте `maxChunkSize` на значение <15KB
3. Включите `trafficMasking.browserBehaviorSimulation = true`
4. Мониторьте логи для отслеживания ротации соединений

Данная реализация предоставляет комплексное решение для обхода российского DPI в рамках существующего протокола xhttp без необходимости создания нового протокола.