# Tarelka-Bot

Telegram-бот на Go: по фото еды считает КБЖУ (калории, белки, жиры, углеводы), ведёт дневник, статистику и даёт рекомендации.

## Требования

- Go 1.24+
- токен Telegram-бота и ключ Gemini API

## Быстрый старт

1. Создай бота у [@BotFather](https://t.me/BotFather), скопируй токен.
2. Возьми ключ Gemini в [Google AI Studio](https://aistudio.google.com/apikey).
3. Узнай свой Telegram user id (например через `@userinfobot`).

```bash
cp .env.example .env
# заполни BOT_TOKEN, GEMINI_API_KEY, ALLOWED_USER_IDS

set -a && source .env && set +a
go run ./cmd/bot
```

## Возможности

- оценка КБЖУ по фото (+ уточнение текстом)
- автосохранение приёма в SQLite-дневник
- кнопки: **Сегодня**, **Неделя**, **Статистика**, **Рекомендации**
- команды: `/today`, `/week`, `/stats`, `/recs`

## Переменные окружения

| Переменная | Описание |
|---|---|
| `BOT_TOKEN` | Токен Telegram-бота |
| `GEMINI_API_KEY` | Ключ Gemini API |
| `GEMINI_MODEL` | Модель (по умолчанию `gemini-3.1-flash-lite`) |
| `ALLOWED_USER_IDS` | Список id через запятую; пусто = все |
| `DATABASE_PATH` | Путь к SQLite (по умолчанию `data/kbju.db`) |
| `TIMEZONE` | Часовой пояс для суток (по умолчанию `Europe/Moscow`) |

## Docker (локально)

```bash
# .env уже должен содержать BOT_TOKEN и GEMINI_API_KEY
docker compose up -d --build
docker compose logs -f
```

Дневник лежит в Docker volume `kbju-data` (не пропадёт при пересборке).

## Деплой на сервер (VPS)

```bash
git clone https://github.com/Landiix/Tarelka-Bot.git
cd Tarelka-Bot
cp .env.example .env
nano .env   # BOT_TOKEN, GEMINI_API_KEY, ALLOWED_USER_IDS, TIMEZONE
docker compose up -d --build
docker compose logs -f bot
```

Публичный порт и домен **не нужны** — бот ходит в Telegram сам (long polling).

## Важно

Оценка приблизительная. Фото на диск не сохраняются — только КБЖУ и названия блюд в SQLite.
