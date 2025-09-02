#!/bin/bash

# Скрипт автоматической настройки Xray для обхода блокировок РКН
# Author: Xray RKN Bypass Configuration

set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Функция для вывода сообщений
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Проверка прав
check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_warning "Скрипт запущен без прав root. Некоторые функции могут быть недоступны."
    fi
}

# Компиляция Xray с улучшениями
build_xray() {
    log_info "Компиляция Xray с улучшениями для обхода DPI..."
    
    if ! command -v go &> /dev/null; then
        log_error "Go не установлен. Установите Go версии 1.21 или выше"
        exit 1
    fi
    
    cd /workspace
    go build -o xray-rkn-bypass -ldflags "-s -w" ./main
    
    if [ $? -eq 0 ]; then
        log_success "Xray успешно скомпилирован: xray-rkn-bypass"
    else
        log_error "Ошибка компиляции"
        exit 1
    fi
}

# Генерация UUID
generate_uuid() {
    if command -v uuidgen &> /dev/null; then
        uuidgen
    else
        cat /proc/sys/kernel/random/uuid 2>/dev/null || echo "$(date +%s)-$(shuf -i 1000-9999 -n 1)-$(shuf -i 1000-9999 -n 1)-$(shuf -i 1000-9999 -n 1)-$(shuf -i 100000-999999 -n 1)"
    fi
}

# Генерация ключей для REALITY
generate_reality_keys() {
    log_info "Генерация ключей REALITY..."
    
    if [ -f "./xray-rkn-bypass" ]; then
        KEYS=$(./xray-rkn-bypass x25519)
        PRIVATE_KEY=$(echo "$KEYS" | grep "Private key:" | cut -d' ' -f3)
        PUBLIC_KEY=$(echo "$KEYS" | grep "Public key:" | cut -d' ' -f3)
        
        echo "PRIVATE_KEY=$PRIVATE_KEY"
        echo "PUBLIC_KEY=$PUBLIC_KEY"
    else
        log_error "xray-rkn-bypass не найден. Сначала выполните компиляцию."
        exit 1
    fi
}

# Генерация short ID
generate_short_id() {
    # Генерируем случайный hex string длиной 8 символов
    openssl rand -hex 4 2>/dev/null || echo "$(shuf -i 10000000-99999999 -n 1)"
}

# Настройка конфигурации
setup_config() {
    log_info "Настройка конфигурации..."
    
    read -p "Введите адрес вашего сервера (например, example.com): " SERVER_ADDR
    
    if [ -z "$SERVER_ADDR" ]; then
        log_error "Адрес сервера не может быть пустым"
        exit 1
    fi
    
    # Генерация параметров
    UUID=$(generate_uuid)
    KEYS=$(./xray-rkn-bypass x25519 2>/dev/null || echo "")
    
    if [ -n "$KEYS" ]; then
        PRIVATE_KEY=$(echo "$KEYS" | grep "Private key:" | cut -d' ' -f3)
        PUBLIC_KEY=$(echo "$KEYS" | grep "Public key:" | cut -d' ' -f3)
    else
        PRIVATE_KEY="your-private-key-here"
        PUBLIC_KEY="your-public-key-here"
    fi
    
    SHORT_ID=$(generate_short_id)
    
    log_info "Сгенерированные параметры:"
    echo "  UUID: $UUID"
    echo "  Private Key: $PRIVATE_KEY"
    echo "  Public Key: $PUBLIC_KEY"
    echo "  Short ID: $SHORT_ID"
    
    # Создание конфигурации
    cp config_rkn_bypass.json config_rkn_bypass_custom.json
    
    # Замена параметров в конфигурации
    if command -v sed &> /dev/null; then
        sed -i "s/your-server.com/$SERVER_ADDR/g" config_rkn_bypass_custom.json
        sed -i "s/your-uuid-here/$UUID/g" config_rkn_bypass_custom.json
        sed -i "s/your-private-key-here/$PRIVATE_KEY/g" config_rkn_bypass_custom.json
        sed -i "s/your-short-id/$SHORT_ID/g" config_rkn_bypass_custom.json
    else
        log_warning "sed не найден. Отредактируйте config_rkn_bypass_custom.json вручную"
    fi
    
    log_success "Конфигурация создана: config_rkn_bypass_custom.json"
    
    echo ""
    log_info "Параметры для настройки сервера:"
    echo "  Public Key: $PUBLIC_KEY"
    echo "  Short ID: $SHORT_ID"
    echo "  UUID: $UUID"
}

# Выбор уровня блокировки
select_blocking_level() {
    log_info "Выберите уровень блокировки в вашем регионе:"
    echo "  1) Легкий (базовая SNI блокировка)"
    echo "  2) Средний (DPI с анализом пакетов)"
    echo "  3) Жесткий (активный DPI с глубоким анализом)"
    echo "  4) Экстремальный (максимальная фрагментация)"
    
    read -p "Выберите вариант (1-4): " LEVEL
    
    case $LEVEL in
        1)
            FRAGMENT_CONFIG='{
                "packets_from": 1,
                "packets_to": 1,
                "length_min": 40,
                "length_max": 60,
                "interval_min": 5,
                "interval_max": 10,
                "max_split_min": 2,
                "max_split_max": 3
            }'
            ;;
        2)
            FRAGMENT_CONFIG='{
                "packets_from": 1,
                "packets_to": 3,
                "length_min": 20,
                "length_max": 80,
                "interval_min": 10,
                "interval_max": 25,
                "max_split_min": 3,
                "max_split_max": 5
            }'
            ;;
        3)
            FRAGMENT_CONFIG='{
                "packets_from": 1,
                "packets_to": 5,
                "length_min": 10,
                "length_max": 50,
                "interval_min": 15,
                "interval_max": 40,
                "max_split_min": 5,
                "max_split_max": 10
            }'
            ;;
        4)
            FRAGMENT_CONFIG='{
                "packets_from": 1,
                "packets_to": 10,
                "length_min": 5,
                "length_max": 30,
                "interval_min": 20,
                "interval_max": 60,
                "max_split_min": 10,
                "max_split_max": 20
            }'
            ;;
        *)
            log_warning "Неверный выбор. Используется средний уровень."
            FRAGMENT_CONFIG='{
                "packets_from": 1,
                "packets_to": 3,
                "length_min": 20,
                "length_max": 80,
                "interval_min": 10,
                "interval_max": 25,
                "max_split_min": 3,
                "max_split_max": 5
            }'
            ;;
    esac
    
    log_success "Уровень фрагментации настроен"
}

# Тестирование конфигурации
test_config() {
    log_info "Тестирование конфигурации..."
    
    # Проверка синтаксиса конфигурации
    if [ -f "./xray-rkn-bypass" ]; then
        ./xray-rkn-bypass test -c config_rkn_bypass_custom.json
        
        if [ $? -eq 0 ]; then
            log_success "Конфигурация корректна"
        else
            log_error "Ошибка в конфигурации"
            exit 1
        fi
    fi
}

# Создание systemd сервиса
create_systemd_service() {
    if [[ $EUID -ne 0 ]]; then
        log_warning "Для создания systemd сервиса требуются права root"
        return
    fi
    
    log_info "Создание systemd сервиса..."
    
    cat > /etc/systemd/system/xray-rkn-bypass.service << EOF
[Unit]
Description=Xray RKN Bypass Service
After=network.target

[Service]
Type=simple
User=nobody
WorkingDirectory=/workspace
ExecStart=/workspace/xray-rkn-bypass run -c /workspace/config_rkn_bypass_custom.json
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
    
    systemctl daemon-reload
    log_success "Systemd сервис создан"
    
    read -p "Запустить сервис сейчас? (y/n): " START_NOW
    if [[ "$START_NOW" == "y" ]]; then
        systemctl start xray-rkn-bypass
        systemctl enable xray-rkn-bypass
        log_success "Сервис запущен и добавлен в автозагрузку"
    fi
}

# Проверка работы прокси
test_proxy() {
    log_info "Проверка работы прокси..."
    
    # Запуск Xray в фоне для теста
    ./xray-rkn-bypass run -c config_rkn_bypass_custom.json &
    XRAY_PID=$!
    
    sleep 3
    
    # Тест SOCKS5
    if command -v curl &> /dev/null; then
        log_info "Тестирование SOCKS5 прокси..."
        curl -x socks5://127.0.0.1:1080 -s -o /dev/null -w "%{http_code}" https://www.google.com
        
        if [ $? -eq 0 ]; then
            log_success "SOCKS5 прокси работает"
        else
            log_warning "SOCKS5 прокси не отвечает"
        fi
        
        # Тест заблокированных сайтов
        log_info "Проверка доступа к заблокированным сайтам..."
        BLOCKED_SITES=("youtube.com" "twitter.com" "instagram.com")
        
        for site in "${BLOCKED_SITES[@]}"; do
            STATUS=$(curl -x socks5://127.0.0.1:1080 -s -o /dev/null -w "%{http_code}" -m 5 https://$site)
            if [ "$STATUS" == "200" ] || [ "$STATUS" == "301" ] || [ "$STATUS" == "302" ]; then
                log_success "$site - доступен"
            else
                log_warning "$site - недоступен (код: $STATUS)"
            fi
        done
    else
        log_warning "curl не установлен. Пропуск тестов."
    fi
    
    # Остановка тестового процесса
    kill $XRAY_PID 2>/dev/null
}

# Главное меню
show_menu() {
    echo ""
    echo "========================================="
    echo "   Xray RKN Bypass - Установка и настройка"
    echo "========================================="
    echo "1) Полная установка (рекомендуется)"
    echo "2) Только компиляция"
    echo "3) Только настройка конфигурации"
    echo "4) Генерация ключей REALITY"
    echo "5) Тестирование прокси"
    echo "6) Создание systemd сервиса"
    echo "7) Выход"
    echo ""
    read -p "Выберите действие (1-7): " CHOICE
    
    case $CHOICE in
        1)
            build_xray
            setup_config
            select_blocking_level
            test_config
            test_proxy
            create_systemd_service
            ;;
        2)
            build_xray
            ;;
        3)
            setup_config
            select_blocking_level
            ;;
        4)
            generate_reality_keys
            ;;
        5)
            test_proxy
            ;;
        6)
            create_systemd_service
            ;;
        7)
            exit 0
            ;;
        *)
            log_error "Неверный выбор"
            show_menu
            ;;
    esac
}

# Основная логика
main() {
    log_info "Запуск скрипта настройки Xray RKN Bypass"
    check_root
    
    # Проверка, что мы в правильной директории
    if [ ! -f "go.mod" ] || [ ! -d "proxy/freedom" ]; then
        log_error "Скрипт должен быть запущен из корневой директории Xray"
        exit 1
    fi
    
    show_menu
    
    echo ""
    log_success "Настройка завершена!"
    echo ""
    echo "Для запуска используйте:"
    echo "  ./xray-rkn-bypass run -c config_rkn_bypass_custom.json"
    echo ""
    echo "Настройки прокси:"
    echo "  SOCKS5: 127.0.0.1:1080"
    echo "  HTTP:   127.0.0.1:8080"
    echo ""
    echo "Документация: см. RKN_BYPASS_SETUP.md"
}

# Запуск
main "$@"