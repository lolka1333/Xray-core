# 🏗️ Shadowsocks: Архитектура и варианты подключения

## 🤔 Ваш вопрос правильный!

На **клиенте** Shadowsocks находится в `inbound`, потому что есть разные архитектуры подключения.

## 📊 Вариант 1: Shadowsocks ЧЕРЕЗ VLESS (туннелирование)

```
SS клиент → Xray клиент → VLESS туннель → Xray сервер → Интернет
```

### Клиент (`shadowsocks-over-vless-client.json`):
```json
"inbounds": [
  {
    "protocol": "shadowsocks",  // ← Принимает SS подключения
    "port": 1081
  }
],
"outbounds": [
  {
    "protocol": "vless"         // ← Отправляет через VLESS
  }
]
```

### Сервер (`shadowsocks-over-vless-server.json`):
```json
"inbounds": [
  {
    "protocol": "vless"         // ← Принимает VLESS туннель
  }
],
"outbounds": [
  {
    "protocol": "freedom"       // ← Отправляет в интернет
  }
]
```

**Преимущества:**
- ✅ SS трафик защищен Post-Quantum криптографией
- ✅ Reality маскировка
- ✅ Двойное шифрование (SS + VLESS)

## 📊 Вариант 2: Отдельный Shadowsocks сервер

```
SS клиент → SS сервер → Интернет
```

### Сервер (`shadowsocks-standalone-server.json`):
```json
"inbounds": [
  {
    "protocol": "shadowsocks",  // ← Принимает SS подключения
    "port": 8388
  }
],
"outbounds": [
  {
    "protocol": "freedom"       // ← Отправляет в интернет
  }
]
```

**Преимущества:**
- ✅ Простота настройки
- ✅ Меньше задержка
- ✅ Стандартная SS архитектура

## 🔄 Логика inbound/outbound:

### На клиенте:
- **inbound** = что принимаем (от других программ)
- **outbound** = куда отправляем (на сервер)

### На сервере:
- **inbound** = что принимаем (от клиентов)
- **outbound** = куда отправляем (в интернет)

## 🎯 Какой вариант выбрать?

### Выберите **Вариант 1** если:
- Нужна максимальная безопасность
- Важна устойчивость к блокировкам
- Есть глубокая инспекция пакетов (DPI)

### Выберите **Вариант 2** если:
- Нужна простота и скорость
- Используете мобильные SS приложения
- Shadowsocks не блокируется в вашем регионе

## 📁 Файлы для каждого варианта:

### Вариант 1 (SS через VLESS):
- `shadowsocks-over-vless-client.json` - клиент
- `shadowsocks-over-vless-server.json` - сервер

### Вариант 2 (отдельный SS):
- `shadowsocks-standalone-server.json` - сервер
- Любой SS клиент (мобильные приложения)

## 🚀 Использование:

### Вариант 1:
```bash
# Сервер
xray -config shadowsocks-over-vless-server.json

# Клиент  
xray -config shadowsocks-over-vless-client.json

# Подключение к клиенту:
# SS: 127.0.0.1:1081
# SOCKS5: 127.0.0.1:1080
# HTTP: 127.0.0.1:8080
```

### Вариант 2:
```bash
# Сервер
xray -config shadowsocks-standalone-server.json

# Подключение напрямую:
# SS: server_ip:8388
```

Теперь понятно? 😊