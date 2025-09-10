#!/bin/bash

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Установка оптимизированного Xray конфига ===${NC}"

# Проверка root прав
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}Этот скрипт должен запускаться с правами root${NC}" 
   exit 1
fi

# Установка необходимых пакетов
echo -e "${YELLOW}Установка необходимых пакетов...${NC}"
apt update
apt install -y curl wget nginx ufw fail2ban

# Установка Xray
echo -e "${YELLOW}Установка Xray...${NC}"
bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install

# Настройка firewall
echo -e "${YELLOW}Настройка firewall...${NC}"
ufw --force enable
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp

# Копирование конфига
echo -e "${YELLOW}Копирование конфигурации...${NC}"
cp server_optimized.json /usr/local/etc/xray/config.json

# Настройка nginx
echo -e "${YELLOW}Настройка nginx...${NC}"
cp nginx_fallback.conf /etc/nginx/sites-available/fallback
ln -sf /etc/nginx/sites-available/fallback /etc/nginx/sites-enabled/
rm -f /etc/nginx/sites-enabled/default

# Создание простой HTML страницы
mkdir -p /var/www/html
cat > /var/www/html/index.html << 'EOF'
<!DOCTYPE html>
<html>
<head>
    <title>Discord - A New Way to Chat with Friends & Communities</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body>
    <h1>Discord</h1>
    <p>A New Way to Chat with Friends & Communities</p>
    <p>Discord is the easiest way to communicate over voice, video, and text.</p>
</body>
</html>
EOF

# Перезапуск сервисов
echo -e "${YELLOW}Перезапуск сервисов...${NC}"
systemctl enable xray
systemctl restart xray
systemctl enable nginx
systemctl restart nginx

# Проверка статуса
echo -e "${YELLOW}Проверка статуса сервисов...${NC}"
if systemctl is-active --quiet xray; then
    echo -e "${GREEN}✓ Xray запущен${NC}"
else
    echo -e "${RED}✗ Ошибка запуска Xray${NC}"
fi

if systemctl is-active --quiet nginx; then
    echo -e "${GREEN}✓ Nginx запущен${NC}"
else
    echo -e "${RED}✗ Ошибка запуска Nginx${NC}"
fi

echo -e "${GREEN}=== Установка завершена ===${NC}"
echo -e "${YELLOW}Не забудьте:${NC}"
echo -e "1. Заменить YOUR_SERVER_IP в клиентском конфиге на IP вашего сервера"
echo -e "2. Сгенерировать новые ключи Reality командой: xray x25519"
echo -e "3. Обновить ключи в обоих конфигах"
echo -e "4. Проверить подключение"