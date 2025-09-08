# Готовые конфигурации Xray с Post-Quantum криптографией

## 📁 Файлы конфигураций

- **`server-config.json`** - конфигурация сервера
- **`client-config.json`** - конфигурация клиента

## 🔐 Используемые ключи

### Сервер:
- **Private Key**: `CDwUEhHCg_I_eT-w29Bllkgbckr5QfLIbPUv4-ZVy3U`
- **MLKEM768 Decryption**: `I9pRa0iDLo2HbQB4a4SIqNAg1iaGvhLx_jWOKeZahKDJDb1h97c0FqJp1V8wZwrfhPmor854yhjj9FqZVlhQRg`
- **MLDSA65 Seed**: `at97S8CG39sznaPX1rrfYuBYEmn96-9KAbaeKMPNeGs`

### Клиент:
- **Public Key**: `okCTU2pbihlxO1Pp9JeZXPAXWnBrjky3YBdaKxHhQVI`
- **MLKEM768 Encryption**: `TjVZa0KhMeGZ-hp6w8Me2dLLEBMKB2A1rfcawaWna3ub...` (полный ключ в конфиге)
- **MLDSA65 Verify**: `1t9524nYvj4Vb0v5IUe-UgrD1JzY9cmiwADRSMzagyk...` (полный ключ в конфиге)

## ⚙️ Настройка сервера

1. **Замените в `server-config.json`:**
   - `example.com` на ваш реальный домен
   - Убедитесь, что порт 443 открыт
   - Настройте веб-сервер на порту 8080 для fallback

2. **Запуск сервера:**
   ```bash
   xray -config server-config.json
   ```

## ⚙️ Настройка клиента

1. **Замените в `client-config.json`:**
   - `YOUR_SERVER_IP` на IP-адрес или домен вашего сервера

2. **Запуск клиента:**
   ```bash
   xray -config client-config.json
   ```

## 🌐 Использование прокси

После запуска клиента будут доступны:
- **SOCKS5 прокси**: `127.0.0.1:1080`
- **HTTP прокси**: `127.0.0.1:8080`

## 🔒 Особенности конфигурации

### Post-Quantum безопасность:
- ✅ **MLKEM768** - квантово-устойчивый обмен ключами
- ✅ **MLDSA65** - квантово-устойчивые цифровые подписи
- ✅ **X25519** - дополнительная защита

### Транспорт:
- **xhttp** - HTTP/2 транспорт для маскировки
- **Reality** - маскировка под реальный сайт
- **XTLS-RPRX-Vision** - высокопроизводительное шифрование

### Маскировка:
- Трафик маскируется под обычные HTTPS запросы к `example.com`
- Используется реальный TLS handshake с целевым сайтом
- Случайное заполнение пакетов (100-1000 байт)

## 🚀 Проверка работы

1. **Проверка конфигураций:**
   ```bash
   xray -test -config server-config.json
   xray -test -config client-config.json
   ```

2. **Проверка подключения:**
   ```bash
   curl -x socks5://127.0.0.1:1080 https://httpbin.org/ip
   ```

## ⚠️ Важные замечания

1. **Домен**: Замените `example.com` на реальный рабочий сайт
2. **Firewall**: Откройте порт 443 на сервере
3. **Версия**: Используйте Xray версии 1.3.0 или новее
4. **Безопасность**: Все ключи уже правильно сопряжены между сервером и клиентом

## 📋 UUID клиента
```
dd272be9-42fa-476d-b88e-face835c8217
```

Этот UUID уже настроен в обеих конфигурациях и готов к использованию.