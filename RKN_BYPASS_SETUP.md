# Настройка Xray для обхода блокировок РКН (DPI)

## Что было сделано

### 1. Улучшенная фрагментация TCP пакетов
- Создан новый модуль `DPIBypassWriter` в `/proxy/freedom/dpi_bypass.go`
- Интеллектуальная фрагментация TLS handshake на уровне SNI
- Фрагментация HTTP запросов на уровне Host заголовка
- Рандомизация размеров фрагментов с экспоненциальным распределением

### 2. Методы обхода DPI

#### A. TLS-специфичные методы:
- **Фрагментация на SNI**: Разбивает TLS ClientHello в критической точке перед полем SNI
- **Фейковые пакеты**: Отправка невалидных TLS записей для сбивания DPI
- **Рандомизация TLS fingerprint**: Изменение порядка cipher suites и curves
- **TLS padding**: Добавление случайного padding к пакетам

#### B. HTTP-специфичные методы:
- **Разбиение Host заголовка**: Фрагментация на уровне "Ho" + "st:"
- **Временные задержки**: Случайные задержки между фрагментами (10-30мс)

#### C. Общие методы:
- **TCP Fast Open**: Отправка данных в SYN пакете
- **Обфускация трафика**: XOR шифрование с случайным ключом
- **Decoy traffic**: Генерация фиктивного трафика
- **Манипуляция TCP параметрами**: TTL, window size, timestamps

### 3. Протокол REALITY
Настроен с оптимальными параметрами для обхода блокировок:
- Маскировка под популярные сайты (Microsoft, Apple, Google)
- Использование chrome fingerprint
- Vision flow для минимизации задержек

## Как использовать

### 1. Компиляция Xray с улучшениями
```bash
cd /workspace
go build -o xray-rkn-bypass ./main
```

### 2. Настройка конфигурации

Отредактируйте файл `config_rkn_bypass.json`:

1. **Замените серверные данные:**
   - `your-server.com` - адрес вашего сервера
   - `your-uuid-here` - UUID пользователя
   - `your-private-key-here` - приватный ключ REALITY
   - `your-short-id` - короткий ID

2. **Настройте фрагментацию под ваш регион:**
```json
"fragment": {
  "packets_from": 1,      // С какого пакета начинать фрагментацию
  "packets_to": 3,        // До какого пакета фрагментировать
  "length_min": 10,       // Минимальный размер фрагмента
  "length_max": 100,      // Максимальный размер фрагмента
  "interval_min": 10,     // Минимальная задержка между фрагментами (мс)
  "interval_max": 30,     // Максимальная задержка между фрагментами (мс)
  "max_split_min": 3,     // Минимальное количество разбиений
  "max_split_max": 5      // Максимальное количество разбиений
}
```

### 3. Запуск
```bash
./xray-rkn-bypass run -c config_rkn_bypass.json
```

### 4. Настройка браузера/системы
- SOCKS5 прокси: `127.0.0.1:1080`
- HTTP прокси: `127.0.0.1:8080`

## Рекомендации по настройке

### Для максимальной эффективности:

1. **Легкие блокировки (SNI-based):**
```json
"fragment": {
  "packets_from": 1,
  "packets_to": 1,
  "length_min": 40,
  "length_max": 60,
  "interval_min": 5,
  "interval_max": 10
}
```

2. **Средние блокировки (DPI с анализом):**
```json
"fragment": {
  "packets_from": 1,
  "packets_to": 3,
  "length_min": 20,
  "length_max": 80,
  "interval_min": 10,
  "interval_max": 25
}
```

3. **Жесткие блокировки (активный DPI):**
```json
"fragment": {
  "packets_from": 1,
  "packets_to": 5,
  "length_min": 10,
  "length_max": 50,
  "interval_min": 15,
  "interval_max": 40,
  "max_split_min": 5,
  "max_split_max": 10
}
```

## Дополнительные оптимизации

### 1. DNS настройки
Используйте DoH (DNS over HTTPS) для обхода DNS блокировок:
- Cloudflare: `https://1.1.1.1/dns-query`
- Google: `https://dns.google/dns-query`
- Для российских сайтов: Яндекс DNS `77.88.8.8`

### 2. TCP оптимизации
```json
"sockopt": {
  "tcpFastOpen": true,        // TCP Fast Open
  "tcpNoDelay": true,          // Отключить Nagle's algorithm
  "tcpKeepAliveInterval": 30,  // Keep-alive интервал
  "tcpMaxSeg": 1360            // Размер TCP сегмента
}
```

### 3. Маршрутизация
- Российские сайты идут через `direct-fragment` (прямое соединение с фрагментацией)
- Заблокированные сайты через `proxy-reality`
- Локальные адреса напрямую без фрагментации

## Тестирование

### Проверка работы фрагментации:
```bash
# Включите debug логи
sed -i 's/"loglevel": "warning"/"loglevel": "debug"/' config_rkn_bypass.json

# Запустите и смотрите логи
./xray-rkn-bypass run -c config_rkn_bypass.json 2>&1 | grep FRAGMENT
```

### Проверка доступности сайтов:
```bash
# Через SOCKS5
curl -x socks5://127.0.0.1:1080 https://www.youtube.com

# Через HTTP
curl -x http://127.0.0.1:8080 https://twitter.com
```

## Troubleshooting

### Если не работает:

1. **Увеличьте фрагментацию:**
   - Уменьшите `length_max` до 30-40
   - Увеличьте `interval_max` до 50-60
   - Увеличьте `max_split_max` до 10-15

2. **Смените маскировочный домен:**
   - Попробуйте другие популярные сайты в `dest`
   - Используйте CDN домены: `cdn.jsdelivr.net`, `cdnjs.cloudflare.com`

3. **Измените fingerprint:**
   - Попробуйте: `firefox`, `safari`, `edge`, `qq`, `ios`, `android`

4. **Добавьте noise пакеты:**
```json
"noises": [
  {
    "apply_to": "ipv4",
    "length_min": 20,
    "length_max": 100,
    "delay_min": 10,
    "delay_max": 30,
    "packet": "SEVMTE8gV09STEQ="  // Base64 encoded fake data
  }
]
```

## Безопасность

⚠️ **Важно:**
- Используйте только доверенные серверы
- Регулярно меняйте UUID и ключи
- Не делитесь конфигурацией с приватными ключами
- Используйте сложные пароли для серверов

## Ссылки

- [Xray-core документация](https://xtls.github.io/)
- [REALITY протокол](https://github.com/XTLS/REALITY)
- [Обсуждение методов обхода](https://github.com/net4people/bbs)