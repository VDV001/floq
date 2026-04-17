# Floq — инструкции для Claude Code

AI-помощник для полного цикла B2B-продаж: inbound (Telegram/Email) + outbound (cold outreach) + AI-квалификация и драфты.

## Стек

- **Backend**: Go 1.26, chi router, pgx/v5, JWT, golang-migrate
- **Frontend**: Next.js 16, React 19, Tailwind 4, shadcn/ui, vitest 4.1
- **AI**: OpenAI + Anthropic SDK, локальный Ollama (в docker compose)
- **DB**: PostgreSQL 18, Redis 8
- **Inbound**: Telegram bot (go-telegram-bot-api), IMAP (emersion/go-imap)
- **Outbound**: Resend (email), SMTP fallback, MTProto (gotd/td) для TG с личного аккаунта
- **Инфра**: docker-compose, OrbStack на dev-машине

## Архитектура

Clean Architecture + DDD. Слои: `domain → usecase → repository → handler`.
Композиция и все адаптеры между контекстами — в `cmd/server/`.

### Bounded contexts (`backend/internal/`)

- **leads** — лиды, messages, qualifications, drafts
- **prospects** — CSV import/export, dedup
- **sequences** — outreach-последовательности, steps, outbound messages
- **sources** — справочник источников (categories + sources), stats
- **inbox** — Telegram bot + IMAP poller, изолирован ACL
- **outbound** — Resend/SMTP/MTProto
- **settings** — настройки пользователя, тестирование AI/IMAP/SMTP/Resend
- **auth** — JWT
- **chat** — AI-ассистент, изолирован от ai через adapter
- **reminders** — cron для stale leads
- **verify** — email/TG верификация
- **parser** — 2GIS, website scraping
- **tgclient** — MTProto (личный TG-аккаунт)

### `cmd/server/`

- `main.go` — composition root
- `helpers.go` — wiring-хелперы (builders, middleware, migrations)
- `adapters.go` — кросс-контекстные адаптеры (leadChecker, prospectRepo, inboxLead, inboxAI, inboxConfig, chatAI)

### Миграции

`backend/migrations/NNN_name.{up,down}.sql`. Запускаются golang-migrate. Текущая версия: 023.

**Грабли**: `NOW()` нельзя использовать в partial index (не IMMUTABLE) — см. chronicles 2026-04-08.

## Правила работы в этом проекте

- **Backend rebuild через OrbStack** после Go-изменений:
  `cd backend && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o server ./cmd/server && cd .. && docker compose up --build backend -d`
- **Коммиты** — логические, conventional commits (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`), scope в скобках опционально.
- **Без Co-Authored-By** в коммитах.
- **Без упоминаний Claude/AI** в issue, PR, коммитах и публичных выводах.
- **Не закрывать issue** без прогона ревью-агента.
- **Исправлять все замечания ревью**, не откладывать как «acceptable trade-off».
- **Не пушить в main напрямую** — только через PR.
- **Рефакторинг**: не трогать код за пределами задачи, не добавлять «улучшения» по пути.
- **Domain-инварианты** — только через фабрики (`NewLead`, `NewProspect`, `NewSource`…) и `TransitionTo`, не прямыми присваиваниями.
- **Кросс-контекстное взаимодействие** — только через порты в домене + адаптеры в `cmd/server/adapters.go`. Импорт пакета другого контекста из domain/usecase запрещён.

## Тесты

- Backend unit: ~350 тестов, 66.4% coverage
- Backend integration (`go:build integration`): 55 тестов, → 79.1% с ними
- Frontend vitest: 115 тестов, 43.2% coverage
- Domain packages: 100% coverage

Запуск:
- `cd backend && go test ./...`
- `cd backend && go test -tags=integration ./...`
- `cd frontend && npm test`

## Фронтенд

Next.js 16 с breaking changes от того, что ты знаешь. Читать `node_modules/next/dist/docs/` перед написанием кода (см. `frontend/AGENTS.md`).

### Структура фронтенда (после issue #13)
- `src/components/{pagename}/` — UI-компоненты per page (constants.ts, секции, карточки)
- `src/hooks/use{PageName}.ts` — state + logic hooks per page
- `src/lib/format.ts` — shared утилиты (getTimeAgo, getInitials)
- Settings: 7 sub-hooks (useSettingsCore, useTelegramBot, useTelegramAccount, useImapSettings, useResendSettings, useSmtpSettings, useAiSettings)
- Page.tsx — тонкий оркестратор (~50-180 строк), только layout + composition

## Open issues

- **#13** — Refactor: разбить монолитные page.tsx (UNCOMMITTED — декомпозиция сделана, ревью пройдено, нужен коммит + PR)

## Память

- Архитектурный журнал: `/Users/daniil/.claude/projects/-Users-daniil-git-floq/memory/chronicles.md` — дописывать при нетривиальных решениях.
- Снапшоты состояния: `project_snapshot_YYYY_MM_DD.md` в той же папке.
- Handoffs: `.claude/handoffs/YYYY-MM-DD_тема.md` в конце сессий >15 мин.
