#!/bin/bash

# Docker build script for lolka1337/3xui
# Исправленная версия с правильным именем репозитория

set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Конфигурация
REPO_NAME="lolka1337/3xui"
TAG="${1:-main}"  # Используем переданный тег или 'main' по умолчанию
DOCKERFILE_PATH=".github/docker/Dockerfile"

echo -e "${YELLOW}=== Docker Build Script для $REPO_NAME ===${NC}"
echo -e "${YELLOW}Тег: $TAG${NC}"
echo ""

# Проверка авторизации Docker
echo -e "${YELLOW}Проверка авторизации Docker...${NC}"
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}Ошибка: Docker не запущен или не доступен${NC}"
    exit 1
fi

# Попытка получить информацию о пользователе (если авторизован)
CURRENT_USER=$(docker info 2>/dev/null | grep "Username:" | awk '{print $2}' || echo "не авторизован")
echo -e "Текущий пользователь Docker: ${GREEN}$CURRENT_USER${NC}"

if [[ "$CURRENT_USER" != "lolka1337" ]]; then
    echo -e "${YELLOW}Рекомендуется авторизоваться под аккаунтом lolka1337${NC}"
    echo -e "${YELLOW}Выполните: docker login${NC}"
    echo ""
fi

# Проверка существования Dockerfile
if [[ ! -f "$DOCKERFILE_PATH" ]]; then
    echo -e "${RED}Ошибка: Dockerfile не найден по пути $DOCKERFILE_PATH${NC}"
    echo -e "${YELLOW}Доступные Dockerfile:${NC}"
    find . -name "Dockerfile*" -type f
    exit 1
fi

echo -e "${GREEN}Dockerfile найден: $DOCKERFILE_PATH${NC}"
echo ""

# Build образа
echo -e "${YELLOW}Начинаем сборку образа...${NC}"
echo -e "Команда: ${GREEN}docker buildx build --push -t $REPO_NAME:$TAG -f $DOCKERFILE_PATH .${NC}"
echo ""

# Выполняем build
if docker buildx build --push -t "$REPO_NAME:$TAG" -f "$DOCKERFILE_PATH" .; then
    echo ""
    echo -e "${GREEN}✅ Успешно собран и отправлен образ: $REPO_NAME:$TAG${NC}"
    echo ""
    echo -e "${YELLOW}Проверить образ можно по ссылке:${NC}"
    echo -e "${GREEN}https://hub.docker.com/r/$REPO_NAME${NC}"
else
    echo ""
    echo -e "${RED}❌ Ошибка при сборке или отправке образа${NC}"
    echo ""
    echo -e "${YELLOW}Возможные решения:${NC}"
    echo -e "1. Проверьте авторизацию: ${GREEN}docker login${NC}"
    echo -e "2. Убедитесь, что репозиторий существует: ${GREEN}https://hub.docker.com/r/$REPO_NAME${NC}"
    echo -e "3. Проверьте права доступа к репозиторию"
    exit 1
fi