# Telegram Link Tracker Bot

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
```


## Запуск проекта

### 1. Сборка проекта

Для сборки всего проекта выполните:

```bash
make build
```

Или соберите компоненты по отдельности:

```bash
make build_bot
make build_scrapper
```

Бинарные файлы будут сохранены в директорию `./bin/`.

### 2. Запуск сервисов

#### Запуск собранных бинарных файлов

1. Запустите Scrapper:
```bash
./bin/scrapper
```

2. Запустите Bot (в отдельном терминале):
```bash
./bin/bot
```