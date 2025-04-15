# Telegram Link Tracker Bot

## Описание
Телеграм-бот для отслеживания обновлений в репозиториях GitHub и вопросах StackOverflow. Бот позволяет добавлять ссылки на ресурсы, отслеживать их обновления и получать уведомления при изменениях.

## Настройка окружения
Настройки проекта хранятся в файле `.env`. В репозитории есть файл `.env.example`, который нужно скопировать и переименовать в `.env`:

```bash
cp .env.example .env
```

Затем откройте файл `.env` и замените значения токенов на ваши:

```
# Общие настройки
TELEGRAM_BOT_TOKEN=your_telegram_bot_token_here
GITHUB_API_TOKEN=your_github_api_token_here
STACKOVERFLOW_API_TOKEN=your_stackoverflow_api_token_here

# Настройки бота
BOT_SERVER_PORT=8080
SCRAPPER_BASE_URL=http://localhost:8081

# Настройки скраппера
SCRAPPER_SERVER_PORT=8081
BOT_BASE_URL=http://localhost:8080
SCHEDULER_CHECK_INTERVAL=1m

# Настройки базы данных
DATABASE_URL=postgres://postgres:postgres@postgres:5432/link_tracker?sslmode=disable
DATABASE_ACCESS_TYPE=SQL
DATABASE_BATCH_SIZE=100
DATABASE_MAX_CONNECTIONS=10

# Настройки PostgreSQL для Docker
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=link_tracker
```

## Запуск проекта с помощью Docker Compose

### Предварительные требования
- Установленный Docker и Docker Compose
- Настроенный файл `.env` с корректными значениями

### Запуск всех сервисов

Для запуска всего приложения, включая базу данных PostgreSQL, миграции и основной сервис, выполните:

```bash
docker-compose up -d
```