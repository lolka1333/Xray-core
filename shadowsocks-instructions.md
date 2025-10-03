# 🔐 Shadowsocks конфигурации

## 📁 Обновленные файлы:

- **`server-config.json`** - сервер с VLESS + Shadowsocks
- **`client-config.json`** - клиент с SOCKS5 + HTTP + Shadowsocks
- **`shadowsocks-standalone.json`** - отдельный Shadowsocks сервер

## 🚀 Доступные прокси после настройки:

### На клиенте:
- **SOCKS5**: `127.0.0.1:1080`
- **HTTP**: `127.0.0.1:8080` 
- **Shadowsocks**: `127.0.0.1:1081`

### На сервере:
- **VLESS (Post-Quantum)**: порт `443`
- **Shadowsocks**: порт `8388`

## ⚙️ Настройка Shadowsocks:

### 1. Смените пароль
В **обеих** конфигурациях замените:
```json
"password": "your-strong-password-here"
```
на сильный пароль, например:
```json
"password": "MyStr0ng!P@ssw0rd2024#Shadowsocks"
```

### 2. Настройки шифрования:
- **Метод**: `aes-256-gcm` (современный и безопасный)
- **Поддержка**: TCP + UDP трафик
- **Порт сервера**: `8388` (стандартный для Shadowsocks)
- **Порт клиента**: `1081`

## 🔧 Варианты использования:

### Вариант 1: Полная конфигурация (VLESS + Shadowsocks)
Используйте обновленные `server-config.json` и `client-config.json`:

**Сервер:**
```bash
xray -config server-config.json
```

**Клиент:**
```bash
xray -config client-config.json
```

### Вариант 2: Только Shadowsocks
Для простого Shadowsocks сервера используйте `shadowsocks-standalone.json`:

```bash
xray -config shadowsocks-standalone.json
```

## 📱 Подключение к Shadowsocks:

### Параметры подключения:
- **Сервер**: IP вашего сервера
- **Порт**: `8388`
- **Метод шифрования**: `aes-256-gcm`
- **Пароль**: ваш пароль
- **Плагин**: не требуется

### Для мобильных приложений:
- **Android**: Shadowsocks для Android
- **iOS**: Shadowrocket, Quantumult X
- **Windows**: Shadowsocks-Windows
- **macOS**: ShadowsocksX-NG

### SS:// ссылка (замените данные):
```
ss://YWVzLTI1Ni1nY206eW91ci1zdHJvbmctcGFzc3dvcmQtaGVyZQ@YOUR_SERVER_IP:8388#MyServer
```

## 🛡️ Безопасность:

1. **Используйте сильный пароль** - минимум 20 символов
2. **Регулярно меняйте пароли**
3. **Используйте разные пароли** для разных серверов
4. **Закройте неиспользуемые порты**

## 🔥 Преимущества каждого протокола:

### VLESS (порт 443):
- ✅ Post-Quantum криптография
- ✅ Reality маскировка
- ✅ Высокая производительность
- ✅ Обход глубокой инспекции пакетов

### Shadowsocks (порт 8388):
- ✅ Простота настройки
- ✅ Широкая поддержка клиентов
- ✅ Стабильность соединения
- ✅ Низкое потребление ресурсов

## 🚨 Важные замечания:

1. **Откройте порты на сервере:**
   ```bash
   # Для VLESS
   ufw allow 443
   # Для Shadowsocks
   ufw allow 8388
   ```

2. **Замените пароли** перед использованием!

3. **Не забудьте** заменить `YOUR_SERVER_IP` в клиентской конфигурации!

4. **Для максимальной безопасности** используйте VLESS с Post-Quantum криптографией