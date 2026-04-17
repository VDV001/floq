# Floq

**AI-помощник для полного цикла продаж малого B2B бизнеса.**

Единый инструмент: входящие лиды + холодный аутрич + AI-мозг.

[![Version](https://img.shields.io/badge/version-0.6.0-blue)](VERSION)
[![CI](https://github.com/VDV001/floq/actions/workflows/ci.yml/badge.svg)](https://github.com/VDV001/floq/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## Зачем это нужно

Малые B2B-команды продаж теряют деньги потому что:

- Лиды приходят из Telegram, Email и других каналов — единой воронки нет
- Менеджеры забывают ответить вовремя — клиент уходит
- Квалификация лида занимает 15 минут вместо 15 секунд
- Холодный аутрич делается вручную, медленно и без персонализации
- Нет инструмента, который связывает исходящий аутрич с входящей воронкой

**Floq решает все пять проблем в одном продукте.**

---

## Что умеет

### Входящие (Inbound)

- **Единый inbox** — Telegram-бот и Email (IMAP) в одном окне
- **AI-квалификация** — потребность, бюджет, сроки, скор 0-100 за секунды
- **AI-черновики ответов** — на русском, с уточняющим вопросом, менеджер редактирует и отправляет
- **Напоминания о фоллоуапах** — если лид молчит 2+ дня, Floq напомнит в Telegram
- **Kanban-воронка** — Новый → Квалифицирован → В диалоге → Фоллоуап → Закрыт

### Исходящие (Outbound)

- **База проспектов** — ручное добавление, CSV-импорт, парсинг из 2GIS
- **Мультиканальные секвенции** — Email (день 0) → Telegram (день 3) → Прозвон (день 8)
- **AI-генерация сообщений** — персонализация по нише, компании, должности, контексту
- **Очередь одобрения** — менеджер подтверждает каждое сообщение перед отправкой
- **Трекинг** — открытия (pixel), ответы, баунсы

### Верификация контактов

- **Свой email-верификатор** (бесплатный, без сторонних API):
  - Синтаксис (RFC), MX-запись, SMTP probe, одноразовые домены, catch-all
  - Скоринг 0-100 → valid / risky / invalid
- **Проверка Telegram username** — resolveUsername через Bot API
- **Запрет отправки** без прохождения верификации

### Парсинг контактов

- **2GIS** — поиск компаний по нише + городу (20 городов)
- **Сайты** — автопоиск email на страницах контактов
- **CSV** — универсальный импорт из любых источников

### AI

- **Pluggable-провайдер** — переключение без изменения кода:
  - Claude (Anthropic) — лучшее качество
  - OpenAI (GPT-4o) — альтернатива
  - Ollama (Llama, Mistral) — бесплатно, локально
- **Динамический выбор** — провайдер и ключ читаются из БД, переключается через UI

---

## Стек технологий

| Слой | Технология |
|------|-----------|
| Backend | Go 1.26, Clean Architecture, chi v5 |
| Frontend | Next.js 16 (App Router), TypeScript, Tailwind CSS v4, shadcn/ui |
| БД | PostgreSQL 18, pgx/v5, golang-migrate |
| Кеш | Redis 8 |
| AI | anthropic-sdk-go, openai-go (OpenAI + Ollama) |
| Telegram | go-telegram-bot-api v5 |
| Email | Resend (отправка), go-imap/v2 (получение) |
| Деплой | Docker Compose |
| CI/CD | GitHub Actions |

---

## Быстрый старт

### Требования

- Docker и Docker Compose
- API-ключ одного из AI-провайдеров (Claude, OpenAI или Ollama)

### 1. Клонировать и настроить

```bash
git clone https://github.com/VDV001/floq.git
cd floq
cp .env.example .env
```

Отредактировать `.env` — как минимум:

```env
JWT_SECRET=ваш-секрет-не-менее-32-символов
AI_PROVIDER=claude
ANTHROPIC_API_KEY=sk-ant-...

# UUID демо-пользователя из миграции 012 (менять не нужно)
OWNER_USER_ID=00000000-0000-0000-0000-000000000001
```

### 2. Запустить

```bash
docker compose up -d
```

Это поднимет PostgreSQL, Redis, Ollama и backend. Миграции применятся автоматически.

> **Ollama**: если планируешь использовать локальную модель, подтяни её:
> ```bash
> docker exec -it floq-ollama-1 ollama pull llama3.2
> ```

### 3. Войти в систему

Миграция `012_seed_data` создаёт демо-пользователя:

| Поле | Значение |
|------|---------|
| Email | `demo@floq.app` |
| Пароль | `demo123` |

### 4. Фронтенд (для разработки)

```bash
cd frontend
npm install
npm run dev
```

Откроется на `http://localhost:3000`, backend API на `http://localhost:8080`.

> Если фронтенд не видит API — проверь, что в `frontend/.env.local` есть:
> ```
> NEXT_PUBLIC_API_URL=http://localhost:8080
> ```

### 5. Без Docker (локально)

```bash
# Терминал 1: поднять инфру
docker compose up -d postgres redis

# Терминал 2: backend
cd backend
go run ./cmd/server

# Терминал 3: frontend
cd frontend
npm run dev
```

Требуется работающий PostgreSQL и Redis (адреса в `.env`).

---

## Архитектура

```
floq/
├── backend/
│   ├── cmd/server/main.go           # Точка входа, DI, роутинг
│   ├── internal/
│   │   ├── leads/                   # Лиды: domain → usecase → repository → handler
│   │   ├── prospects/               # Проспекты: CRUD, CSV-импорт
│   │   ├── sequences/               # Секвенции: шаги, запуск, очередь
│   │   ├── inbox/                   # Telegram бот, Email IMAP poller
│   │   ├── outbound/                # Email-отправка через Resend
│   │   ├── verify/                  # Верификация email и Telegram
│   │   ├── parser/                  # 2GIS API, парсинг сайтов
│   │   ├── ai/                      # Provider interface + Claude/OpenAI/Ollama
│   │   ├── auth/                    # JWT: регистрация, логин, refresh
│   │   ├── settings/                # Настройки пользователя из БД
│   │   ├── reminders/               # Cron: напоминания о молчащих лидах
│   │   ├── notify/                  # Telegram-уведомления менеджеру
│   │   ├── config/                  # Чтение .env
│   │   ├── db/                      # TxManager, транзакции
│   │   └── httputil/                # JSON-ответы, контекст запроса
│   └── migrations/                  # 16 SQL-миграций (up + down)
├── frontend/
│   ├── src/app/                     # 11 страниц (App Router)
│   ├── src/components/              # 18+ компонентов (shadcn/ui)
│   └── src/lib/api.ts               # API-клиент
├── docker-compose.yml
├── .env.example
└── .github/workflows/ci.yml        # CI: build + test + lint
```

### Принципы

- **Clean Architecture** — handler → usecase → repository → domain
- **Domain-Driven Design** — value objects, entities, port interfaces
- **Зависимости внутрь** — usecase не знает про HTTP или БД
- **Интерфейсы на границах** — все внешние зависимости через порты
- **DTO на выходе** — domain-сущности не имеют json-тегов, маппятся в DTO в handler

---

## API

40+ эндпоинтов. Основные группы:

| Группа | Примеры | Авторизация |
|--------|---------|-------------|
| Auth | `POST /api/auth/login`, `/register`, `/refresh` | Публичные |
| Leads | `GET /api/leads`, `POST /api/leads/:id/qualify` | JWT |
| Prospects | `GET /api/prospects`, `POST /api/prospects/import` | JWT |
| Sequences | `POST /api/sequences/:id/launch`, `/steps` | JWT |
| Outbound | `GET /api/outbound/queue`, `/approve`, `/stats` | JWT |
| Verify | `POST /api/verify/email`, `/batch` | JWT |
| Parser | `POST /api/parser/twogis`, `/website` | JWT |
| Settings | `GET /api/settings`, `PUT /api/settings` | JWT |
| Tracking | `GET /api/track/open/:id` | Публичный (pixel) |

Полный список в [PROJECT_FOUNDATION.md](PROJECT_FOUNDATION.md#8-api-routes).

---

## Переменные окружения

| Переменная | Описание | Обязательна |
|-----------|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string | Да |
| `JWT_SECRET` | Секрет для подписи JWT-токенов | Да |
| `OWNER_USER_ID` | UUID основного пользователя | Да |
| `AI_PROVIDER` | `claude` / `openai` / `ollama` | Да |
| `ANTHROPIC_API_KEY` | API-ключ Claude | Если claude |
| `OPENAI_API_KEY` | API-ключ OpenAI | Если openai |
| `OPENAI_MODEL` | Модель OpenAI (по умолчанию gpt-4o) | Нет |
| `OLLAMA_BASE_URL` | URL Ollama (по умолчанию localhost:11434) | Если ollama |
| `OLLAMA_MODEL` | Модель Ollama (по умолчанию llama3.2) | Если ollama |
| `TELEGRAM_BOT_TOKEN` | Токен Telegram бота для входящих сообщений | Нет |
| `NOTIFY_CHAT_ID` | Telegram chat ID для уведомлений о stale-лидах | Нет |
| `RESEND_API_KEY` | API-ключ Resend для исходящей почты | Нет |
| `SMTP_HOST` | SMTP-сервер (альтернатива Resend) | Нет |
| `SMTP_PORT` | Порт SMTP (по умолчанию 465) | Нет |
| `SMTP_USER` | Логин SMTP | Нет |
| `SMTP_PASSWORD` | Пароль SMTP | Нет |
| `SMTP_FROM` | Email отправителя | Нет |
| `IMAP_HOST` | IMAP-сервер для входящей почты (imap.gmail.com) | Нет |
| `IMAP_PORT` | Порт IMAP (по умолчанию 993) | Нет |
| `IMAP_USER` | Логин IMAP | Нет |
| `IMAP_PASSWORD` | Пароль IMAP (для Gmail — App Password) | Нет |
| `GROQ_API_KEY` | API-ключ Groq | Нет |
| `GROQ_MODEL` | Модель Groq (по умолчанию openai/gpt-oss-120b) | Нет |
| `TWOGIS_API_KEY` | API-ключ 2GIS для парсера | Нет |
| `BOOKING_LINK` | Ссылка на Calendly/Cal.com | Нет |
| `APP_BASE_URL` | Публичный URL бэкенда (нужен для tracking-пикселей) | Нет |
| `SENDER_NAME` | Имя отправителя в outreach-сообщениях | Нет |
| `SENDER_COMPANY` | Компания отправителя | Нет |
| `SENDER_PHONE` | Телефон отправителя | Нет |
| `SENDER_WEBSITE` | Сайт отправителя | Нет |
| `STALE_DAYS` | Дней без ответа до напоминания (по умолчанию 2) | Нет |

Полный шаблон: [.env.example](.env.example)

---

## Разработка

### Запустить тесты

```bash
cd backend
go test -race ./...
```

### Добавить миграцию

```bash
# Создать файлы миграции
touch backend/migrations/017_описание.up.sql
touch backend/migrations/017_описание.down.sql
```

Миграции применяются автоматически при старте сервера.

### Линтинг фронтенда

```bash
cd frontend
npm run lint
```

---

## Лицензия

[MIT](LICENSE)
