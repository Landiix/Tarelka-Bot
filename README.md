# food-kbju-bot

Telegram-бот на Go: фото еды → приблизительная оценка КБЖУ, дневник питания, статистика и рекомендации.

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

На любом Linux с Docker (Timeweb, Selectel, Hetzner, Beget и т.д.):

```bash
# на сервере
git clone <твой-репо> food-kbju-bot && cd food-kbju-bot
nano .env   # BOT_TOKEN, GEMINI_API_KEY, ALLOWED_USER_IDS, TIMEZONE
docker compose up -d --build
docker compose logs -f bot
```

Публичный порт и домен **не нужны** — бот ходит в Telegram сам (long polling).

Остановить Mac-хостинг: `Ctrl+C` / убить локальный `go run`.

## Важно

Оценка приблизительная. Фото на диск не сохраняются — только КБЖУ и названия блюд в SQLite.
