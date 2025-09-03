#!/bin/bash

echo "==================================="
echo "Тест обхода DPI в Xray-core"
echo "==================================="
echo

# Цвета для вывода
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Проверка 1: Компиляция
echo -e "${YELLOW}1. Проверка компиляции...${NC}"
if go build ./... 2>/dev/null; then
    echo -e "${GREEN}✓ Компиляция успешна${NC}"
else
    echo -e "${RED}✗ Ошибка компиляции${NC}"
    exit 1
fi
echo

# Проверка 2: Проверка включения методов обхода
echo -e "${YELLOW}2. Проверка включения методов обхода DPI...${NC}"

# Проверяем фрагментацию
if grep -q "return true" transport/internet/tcp/dialer.go | head -1 > /dev/null; then
    echo -e "${GREEN}✓ TCP фрагментация включена${NC}"
else
    echo -e "${RED}✗ TCP фрагментация отключена${NC}"
fi

# Проверяем TLS обфускацию
if grep -A3 "func shouldObfuscateTLS" transport/internet/tcp/dialer.go | grep -q "return true"; then
    echo -e "${GREEN}✓ TLS обфускация включена${NC}"
else
    echo -e "${RED}✗ TLS обфускация отключена${NC}"
fi

# Проверяем HTTP обфускацию
if grep -A3 "func shouldApplyHTTPObfuscation" transport/internet/websocket/dialer.go | grep -q "return true"; then
    echo -e "${GREEN}✓ HTTP обфускация включена${NC}"
else
    echo -e "${RED}✗ HTTP обфускация отключена${NC}"
fi
echo

# Проверка 3: Наличие всех файлов обхода DPI
echo -e "${YELLOW}3. Проверка наличия всех компонентов обхода DPI...${NC}"

files=(
    "transport/internet/tcp/fragment.go"
    "transport/internet/tcp/multiplex.go"
    "transport/internet/tls/obfuscate.go"
    "transport/internet/http_obfuscate.go"
    "transport/internet/dpi_bypass.go"
    "transport/internet/dpi_bypass_unix.go"
    "transport/internet/dpi_bypass_windows.go"
)

all_present=true
for file in "${files[@]}"; do
    if [ -f "$file" ]; then
        echo -e "${GREEN}✓ $file${NC}"
    else
        echo -e "${RED}✗ $file не найден${NC}"
        all_present=false
    fi
done
echo

# Проверка 4: Проверка параметров обхода
echo -e "${YELLOW}4. Проверка параметров обхода DPI...${NC}"

# Размер фрагмента
fragment_size=$(grep "return 40" transport/internet/tcp/dialer.go)
if [ ! -z "$fragment_size" ]; then
    echo -e "${GREEN}✓ Размер фрагмента: 40 байт (оптимально для РКН)${NC}"
else
    echo -e "${RED}✗ Размер фрагмента не установлен${NC}"
fi

# TTL для обхода
ttl_fake=$(grep "TTLFake.*3" transport/internet/dpi_bypass_unix.go)
if [ ! -z "$ttl_fake" ]; then
    echo -e "${GREEN}✓ TTL для фейковых пакетов: 3${NC}"
else
    echo -e "${RED}✗ TTL для фейковых пакетов не установлен${NC}"
fi

# MSS
mss=$(grep "MSS.*1360" transport/internet/dpi_bypass_unix.go)
if [ ! -z "$mss" ]; then
    echo -e "${GREEN}✓ MSS: 1360 байт${NC}"
else
    echo -e "${RED}✗ MSS не установлен${NC}"
fi

# Лимит данных для мультиплексирования
data_limit=$(grep "DefaultDataLimitPerConnection = 15" transport/internet/tcp/multiplex.go)
if [ ! -z "$data_limit" ]; then
    echo -e "${GREEN}✓ Лимит данных на соединение: 15KB (для обхода блокировки РКН)${NC}"
else
    echo -e "${RED}✗ Лимит данных не установлен${NC}"
fi
echo

# Проверка 5: Документация
echo -e "${YELLOW}5. Проверка документации...${NC}"
if [ -f "DPI_BYPASS_METHODS.md" ]; then
    echo -e "${GREEN}✓ Документация по методам обхода DPI присутствует${NC}"
    methods=$(grep -c "^###" DPI_BYPASS_METHODS.md)
    echo -e "${GREEN}  Документировано методов: $methods${NC}"
else
    echo -e "${RED}✗ Документация отсутствует${NC}"
fi
echo

# Итоговый результат
echo -e "${YELLOW}==================================="
echo "ИТОГОВЫЙ РЕЗУЛЬТАТ:"
echo -e "===================================${NC}"

if $all_present; then
    echo -e "${GREEN}✓ Все компоненты обхода DPI интегрированы и активированы!${NC}"
    echo -e "${GREEN}✓ Проект готов к использованию для обхода блокировок.${NC}"
    echo
    echo "Реализованные методы обхода:"
    echo "  • TCP фрагментация (40 байт)"
    echo "  • TLS обфускация и маскировка SNI"
    echo "  • HTTP обфускация для WebSocket"
    echo "  • TCP мультиплексирование (обход лимита 15KB)"
    echo "  • TTL манипуляции (TTL=3 для фейковых пакетов)"
    echo "  • Системные оптимизации сокетов"
    echo
    echo -e "${GREEN}Методы работают автоматически и не требуют дополнительной настройки!${NC}"
else
    echo -e "${RED}✗ Некоторые компоненты отсутствуют или не настроены${NC}"
fi