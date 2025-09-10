# Руководство по устранению неполадок Xray-core

## 🚨 Основные проблемы и решения

### 1. Ошибка "stream-one mode is not allowed"

**Симптомы:**
```
transport/internet/splithttp: stream-one mode is not allowed
```

**Причина:** Несоответствие режимов XHTTP между клиентом и сервером.

**Решение:**
1. Используйте **одинаковые режимы** на клиенте и сервере:
   - `"mode": "stream-up"` (рекомендуется)
   - `"mode": "packet-up"` (для обхода блокировок)
   - `"mode": "auto"` (только если сервер поддерживает все режимы)

### 2. Ошибка "failed to POST" / Timeout

**Симптомы:**
```
failed to POST https://www.microsoft.com/api/v2/
wsarecv: A connection attempt failed because the connected party did not properly respond
```

**Возможные причины:**
1. **TCP Fast Open не поддерживается сервером**
2. **Блокировка провайдером**
3. **Неправильные Reality настройки**
4. **Проблемы с сетью/фаерволом**

**Решения:**

#### A. Отключить TCP Fast Open
```json
"sockopt": {
  "tcpFastOpen": false,  // Изменить на false
  "tcpMptcp": true
}
```

#### B. Проверить доступность сервера
```bash
# Проверить подключение к серверу
telnet 51.89.39.34 8443

# Проверить Reality домен
curl -I https://www.microsoft.com
```

#### C. Включить отладку Reality
```json
"realitySettings": {
  "show": true,  // Добавить для отладки
  // ... остальные настройки
}
```

### 3. Проблемы с портами

**Симптомы:**
- Соединение не устанавливается
- Timeout при подключении

**Проверки:**
1. **Убедитесь, что порты совпадают:**
   - Сервер слушает: `"port": 8443`
   - Клиент подключается: `"port": 8443`

2. **Проверьте фаервол на сервере:**
```bash
# Ubuntu/Debian
sudo ufw allow 8443/tcp

# CentOS/RHEL
sudo firewall-cmd --add-port=8443/tcp --permanent
sudo firewall-cmd --reload
```

### 4. Проблемы с ключами Reality

**Симптомы:**
- HTTP 400 ошибки
- Соединение сразу обрывается

**Решение:**
Убедитесь, что ключи совпадают:

**Сервер:**
```json
"privateKey": "CDwUEhHCg_I_eT-w29Bllkgbckr5QfLIbPUv4-ZVy3U"
```

**Клиент:**
```json
"publicKey": "okCTU2pbihlxO1Pp9JeZXPAXWnBrjky3YBdaKxHhQVI"
```

## 📋 Рекомендуемые рабочие конфигурации

### Сервер (`working_server.json`)
- Режим: `stream-up`
- TCP Fast Open: отключен
- Порт: 8443
- Reality: Microsoft.com

### Клиент (`working_client.json`)
- Режим: `stream-up` (совпадает с сервером)
- TCP Fast Open: отключен
- Меньшие размеры пакетов для стабильности

## 🔧 Диагностические команды

### На сервере:
```bash
# Проверить статус службы
sudo systemctl status xray

# Посмотреть логи в реальном времени
sudo journalctl -u xray -f

# Проверить, слушает ли порт
sudo ss -tuln | grep 8443

# Тест конфигурации
xray -test -config /opt/xray/config.json
```

### На клиенте:
```bash
# Windows
./xray -test -config .\working_client.json

# Проверить подключение к серверу
telnet 51.89.39.34 8443
```

## 🌐 Проверка работы

1. **Запустить сервер** с `working_server.json`
2. **Запустить клиента** с `working_client.json`
3. **Настроить браузер** на прокси `127.0.0.1:10999`
4. **Проверить IP**: https://whatismyipaddress.com/

## 📝 Логи для анализа

При возникновении проблем, соберите логи:

**Сервер:**
```bash
sudo journalctl -u xray --since "10 minutes ago" > server_logs.txt
```

**Клиент:**
```bash
./xray -config .\working_client.json > client_logs.txt 2>&1
```

## ⚡ Быстрые исправления

1. **Замените `auto` на `stream-up`** в обеих конфигурациях
2. **Отключите TCP Fast Open** (`tcpFastOpen: false`)
3. **Используйте меньшие размеры пакетов** (`8192-16384`)
4. **Проверьте доступность Microsoft.com** с сервера
5. **Убедитесь в правильности IP-адреса** сервера в клиенте

## 🔍 Дополнительная диагностика

Если проблемы продолжаются:

1. **Попробуйте другой домен** вместо Microsoft.com (например, `cloudflare.com`)
2. **Измените порт** (например, на 443 или 80)
3. **Отключите Reality** временно для тестирования
4. **Используйте режим `stream-one`** для простого тестирования