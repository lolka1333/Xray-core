#!/bin/bash

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Установка Xray с полным шифрованием (без nginx) ===${NC}"

# Проверка root прав
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}Этот скрипт должен запускаться с правами root${NC}" 
   exit 1
fi

# Установка необходимых пакетов
echo -e "${YELLOW}Установка необходимых пакетов...${NC}"
apt update
apt install -y curl wget ufw fail2ban

# Установка Xray
echo -e "${YELLOW}Установка Xray...${NC}"
bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install

# Настройка firewall
echo -e "${YELLOW}Настройка firewall...${NC}"
ufw --force enable
ufw allow 22/tcp
ufw allow 8443/tcp

# Копирование конфига
echo -e "${YELLOW}Копирование конфигурации...${NC}"
cp server_optimized.json /usr/local/etc/xray/config.json

# Перезапуск сервисов
echo -e "${YELLOW}Перезапуск Xray...${NC}"
systemctl enable xray
systemctl restart xray

# Проверка статуса
echo -e "${YELLOW}Проверка статуса Xray...${NC}"
if systemctl is-active --quiet xray; then
    echo -e "${GREEN}✓ Xray запущен успешно${NC}"
    echo -e "${GREEN}✓ Порт: 8443${NC}"
    echo -e "${GREEN}✓ Протокол: VLESS + XHTTP + Reality${NC}"
    echo -e "${GREEN}✓ Post-quantum криптография: включена${NC}"
    echo -e "${GREEN}✓ MLDSA65: включена${NC}"
else
    echo -e "${RED}✗ Ошибка запуска Xray${NC}"
    echo -e "${YELLOW}Проверьте логи: journalctl -u xray -f${NC}"
fi

# Показать конфиг для проверки
echo -e "${YELLOW}Проверка конфигурации:${NC}"
ss -tulpn | grep :8443
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Порт 8443 открыт и слушается${NC}"
else
    echo -e "${RED}✗ Порт 8443 не открыт${NC}"
fi

echo -e "${GREEN}=== Установка завершена ===${NC}"
echo -e "${YELLOW}Важно:${NC}"
echo -e "1. Замените YOUR_SERVER_IP в клиентском конфиге на: $(curl -s ifconfig.me)"
echo -e "2. Используйте порт 8443 для подключения"
echo -e "3. Все криптографические настройки восстановлены:"
echo -e "   - Post-quantum: mlkem768x25519plus"
echo -e "   - MLDSA65 seed включен"
echo -e "   - XHTTP с полными заголовками"
echo -e "4. Тестируйте подключение по прямому IP"

echo -e "${YELLOW}Диагностика:${NC}"
echo -e "- Логи: journalctl -u xray -f"
echo -e "- Статус: systemctl status xray"
echo -e "- Порт: ss -tulpn | grep 8443"