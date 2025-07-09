# Обход российского DPI в протоколе xhttp

## Обзор

Данная реализация добавляет возможности обхода российского DPI (Deep Packet Inspection) в существующий протокол xhttp без создания нового протокола. Все изменения полностью совместимы с существующим кодом xray-core.

## Проблема

Российские интернет-провайдеры внедрили жесткие ограничения на TLS 1.3 соединения:
- Лимит 15-20KB на соединение к зарубежным серверам
- Энтропийная классификация трафика
- Анализ поведения соединений
- Блокировка популярных хостинг-провайдеров

## Решение

### Основные компоненты

1. **ResponseFragmenter** - Автоматическое разбиение ответов на фрагменты <15KB
2. **ConnectionManager** - Управление ротацией соединений
3. **TrafficMasker** - Маскировка под обычный браузерный трафик
4. **CDNHopper** - Переключение между CDN-провайдерами
5. **EntropyReducer** - Снижение энтропии для обхода классификации

### Модифицированные файлы

- `transport/internet/splithttp/config.proto` - Расширенная конфигурация
- `transport/internet/splithttp/config.go` - Вспомогательные функции
- `transport/internet/splithttp/dpi_bypass.go` - Основная реализация
- `transport/internet/splithttp/client.go` - Клиентская интеграция
- `transport/internet/splithttp/hub.go` - Серверная интеграция
- `transport/internet/splithttp/dpi_bypass_test.go` - Тесты
- `transport/internet/splithttp/config_examples.json` - Примеры конфигурации

## Использование

### Минимальная конфигурация

```json
{
  "xhttpSettings": {
    "path": "/",
    "host": "example.com",
    "dpiBypass": {
      "enabled": true,
      "responseFragmentation": {
        "enabled": true,
        "maxChunkSize": 14000
      },
      "trafficMasking": {
        "userAgentRotation": true,
        "browserBehaviorSimulation": true
      }
    }
  }
}
```

### Полная конфигурация

```json
{
  "xhttpSettings": {
    "path": "/api/v1/",
    "host": "example.com",
    "mode": "packet-up",
    "dpiBypass": {
      "enabled": true,
      "responseFragmentation": {
        "enabled": true,
        "maxChunkSize": 14000,
        "randomDelay": {"from": 100, "to": 500},
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
      "cdnHopping": {
        "enabled": true,
        "providers": ["cloudflare", "gcore", "aws"],
        "rotationInterval": 60,
        "failoverThreshold": 2
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

## Тестирование

Запустите тесты для проверки функциональности:

```bash
cd transport/internet/splithttp
go test -v -run TestDPIBypass
go test -v -bench=.
```

## Преимущества

1. **Совместимость** - Работает с существующим кодом xray-core
2. **Гибкость** - Каждая функция может быть настроена или отключена
3. **Эффективность** - Минимальные накладные расходы
4. **Специфичность** - Оптимизировано для российских условий
5. **Безопасность** - Не нарушает безопасность протокола

## Отладка

Для включения отладочной информации:

1. Включите `dpiBypass.enabled = true`
2. Используйте логирование xray-core для отслеживания ротации соединений
3. Мониторьте размер фрагментов ответов

## Поддержка

Данная реализация предоставляет фундамент для дальнейшего развития методов обхода DPI в рамках протокола xhttp. Все функции можно расширять и улучшать по мере необходимости.

---

*Примечание: Данная реализация создана для образовательных и исследовательских целей. Использование должно соответствовать местному законодательству.*