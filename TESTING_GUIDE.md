# Пошаговое руководство по тестированию конфигураций

## 🚨 Текущая проблема

**Симптомы:**
- Клиент подключается в режиме `stream-up`
- GET запросы к Microsoft.com получают `EOF` ошибки
- Сервер разрывает соединения

**Диагноз:** Microsoft.com блокирует запросы к несуществующему API `/api/v2/`

## 🔧 Варианты решения

### Вариант 1: Простая XHTTP конфигурация
**Файлы:** `simple_server.json` + `simple_client.json`

**Изменения:**
- Упрощенный путь: `/` вместо `/api/v2/`
- Убрана сложная криптография (`decryption: "none"`)
- Режим `auto` для автоматического выбора

### Вариант 2: Cloudflare домен
**Файлы:** `cloudflare_server.json` + `cloudflare_client.json`

**Изменения:**
- Домен: `www.cloudflare.com` (более стабильный)
- Путь: `/api/` (существует у Cloudflare)
- Cloudflare не блокирует подобные запросы

### Вариант 3: WebSocket транспорт
**Файлы:** `ws_server.json` + `ws_client.json`

**Изменения:**
- Транспорт: `ws` вместо `xhttp`
- Убран `flow: "xtls-rprx-vision"` (не совместим с WS)
- Более стабильный протокол

## 📋 Пошаговое тестирование

### Шаг 1: Остановить текущие службы
```bash
# На сервере
sudo systemctl stop xray

# На клиенте
Ctrl+C (остановить xray)
```

### Шаг 2: Тест простой XHTTP конфигурации
```bash
# Сервер
sudo cp simple_server.json /opt/xray/config.json
sudo systemctl start xray
sudo journalctl -u xray -f

# Клиент (в отдельном терминале)
./xray -config .\simple_client.json
```

**Ожидаемый результат:** Нет ошибок `EOF`, соединение стабильное.

### Шаг 3: Если не работает - тест Cloudflare
```bash
# Сервер
sudo systemctl stop xray
sudo cp cloudflare_server.json /opt/xray/config.json
sudo systemctl start xray

# Клиент
./xray -config .\cloudflare_client.json
```

### Шаг 4: Если не работает - тест WebSocket
```bash
# Сервер
sudo systemctl stop xray
sudo cp ws_server.json /opt/xray/config.json
sudo systemctl start xray

# Клиент
./xray -config .\ws_client.json
```

## 🔍 Диагностика проблем

### Проверка доступности целевых доменов с сервера
```bash
# Проверить Microsoft.com
curl -I https://www.microsoft.com/

# Проверить Cloudflare.com
curl -I https://www.cloudflare.com/api/

# Проверить конкретный путь
curl -I https://www.microsoft.com/api/v2/
```

### Проверка Reality настроек
```bash
# Включить отладку в конфигурации
"realitySettings": {
  "show": true,  // Добавить эту строку
  // ... остальные настройки
}
```

### Проверка сетевого подключения
```bash
# С клиента проверить доступность сервера
telnet 51.89.39.34 8443

# Проверить DNS разрешение
nslookup www.microsoft.com
nslookup www.cloudflare.com
```

## ⚡ Быстрые исправления

1. **Смените путь на корневой:**
   ```json
   "path": "/"
   ```

2. **Смените домен на Cloudflare:**
   ```json
   "host": "www.cloudflare.com"
   "dest": "www.cloudflare.com:443"
   ```

3. **Упростите криптографию:**
   ```json
   "decryption": "none"
   "encryption": "none"
   ```

4. **Используйте WebSocket:**
   ```json
   "network": "ws"
   "flow": ""  // Убрать xtls-rprx-vision
   ```

## 🎯 Рекомендуемый порядок тестирования

1. **simple_** конфигурации (самые простые)
2. **cloudflare_** конфигурации (другой домен)
3. **ws_** конфигурации (другой транспорт)

## 📊 Ожидаемые результаты

### Успешное подключение:
```
[Info] transport/internet/splithttp: XHTTP is dialing to tcp:51.89.39.34:8443, mode auto
[Info] proxy/vless/outbound: tunneling request to tcp:example.com:443 via 51.89.39.34:8443
```

### Ошибка (нужно менять конфигурацию):
```
[Info] transport/internet/splithttp: failed to GET ... EOF
```

## 🔧 Финальная рекомендация

Если все варианты не работают, возможные причины:
1. **Блокировка провайдером** - попробуйте другой порт (443, 80)
2. **Проблемы с Reality ключами** - сгенерируйте новые
3. **Фаервол на сервере** - проверьте правила

Начните с **simple_** конфигураций - они имеют наибольшие шансы на успех!