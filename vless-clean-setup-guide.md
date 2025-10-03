# 🚀 Чистые VLESS конфигурации (без Shadowsocks)

## 📁 Созданные файлы:

- **`vless-server-clean.json`** - сервер только с VLESS
- **`vless-client-clean.json`** - клиент с mixed протоколом

## ✨ Особенности чистой конфигурации:

### 🎯 **Максимальная простота:**
- Только VLESS протокол
- Один mixed порт на клиенте
- Минимум настроек

### 🔐 **Полная защита:**
- ✅ **Post-Quantum криптография** (MLKEM768 + MLDSA65)
- ✅ **Reality маскировка** под реальный сайт
- ✅ **XTLS-RPRX-Vision** высокопроизводительное шифрование
- ✅ **xhttp транспорт** с HTTP/2

### 🌐 **Универсальный клиент:**
- **Порт 1080** поддерживает:
  - SOCKS4/4a/5
  - HTTP прокси
  - Автоматическое определение протокола

## ⚙️ Настройка:

### 1. **Сервер** (`vless-server-clean.json`):
```bash
# Запуск сервера
xray -config vless-server-clean.json

# Порт: 443 (VLESS + Reality)
```

### 2. **Клиент** (`vless-client-clean.json`):

**Замените адрес сервера:**
```json
"address": "YOUR_SERVER_IP"  // ← Замените на IP вашего сервера
```

**Запуск клиента:**
```bash
xray -config vless-client-clean.json

# Доступные прокси:
# SOCKS5: socks5://127.0.0.1:1080
# HTTP: http://127.0.0.1:1080
```

## 🔧 Используемые ключи:

### 🔐 **Сервер:**
- **Private Key**: `CDwUEhHCg_I_eT-w29Bllkgbckr5QfLIbPUv4-ZVy3U`
- **MLKEM768**: `I9pRa0iDLo2HbQB4a4SIqNAg1iaGvhLx_jWOKeZahKDJDb1h97c0FqJp1V8wZwrfhPmor854yhjj9FqZVlhQRg`
- **MLDSA65**: `at97S8CG39sznaPX1rrfYuBYEmn96-9KAbaeKMPNeGs`

### 🔐 **Клиент:**
- **Public Key**: `okCTU2pbihlxO1Pp9JeZXPAXWnBrjky3YBdaKxHhQVI`
- **MLKEM768**: `TjVZa0KhMeGZ-hp6w8Me2dLLEBMKB2A1rfcawaWna3ub...`
- **MLDSA65**: `1t9524nYvj4Vb0v5IUe-UgrD1JzY9cmiwADRSMzagyk...`

## 🎯 **Подключение приложений:**

### **Firefox:**
```
network.proxy.type: 1
network.proxy.socks: 127.0.0.1
network.proxy.socks_port: 1080
network.proxy.socks_version: 5
```

### **Chrome:**
```bash
chrome --proxy-server=socks5://127.0.0.1:1080
# или
chrome --proxy-server=http://127.0.0.1:1080
```

### **Системный прокси (Windows):**
```
SOCKS5: 127.0.0.1:1080
HTTP: 127.0.0.1:1080
```

## ✅ **Проверка работы:**

### 1. **Тест конфигураций:**
```bash
# Сервер
xray -test -config vless-server-clean.json

# Клиент
xray -test -config vless-client-clean.json
```

### 2. **Проверка подключения:**
```bash
# Через SOCKS5
curl -x socks5://127.0.0.1:1080 https://httpbin.org/ip

# Через HTTP
curl -x http://127.0.0.1:1080 https://httpbin.org/ip
```

## 🛡️ **Безопасность:**

### **Reality маскировка:**
- Трафик выглядит как обычные HTTPS запросы к `example.com`
- Используется реальный TLS handshake
- Невозможно отличить от обычного веб-трафика

### **Post-Quantum защита:**
- Устойчивость к атакам квантовых компьютеров
- Современные алгоритмы MLKEM768 и MLDSA65
- Двойное шифрование (PQ + классическое)

## 🚨 **Важные настройки:**

1. **Откройте порт 443** на сервере:
   ```bash
   ufw allow 443
   ```

2. **Замените `example.com`** на реальный домен для лучшей маскировки

3. **Настройте fallback** на порту 8080 (веб-сервер для маскировки)

## 🎉 **Преимущества чистой VLESS конфигурации:**

- 🔥 **Максимальная производительность**
- 🔥 **Простота настройки**
- 🔥 **Минимум точек отказа**
- 🔥 **Современная криптография**
- 🔥 **Универсальная совместимость**

**Готово к использованию!** 🚀