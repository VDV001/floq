# Floq — Roadmap

> AI-помощник для полного цикла B2B-продаж: inbound (Telegram/Email) + outbound (cold outreach) + AI-квалификация и драфты.
> Обновлено: 2026-05-20

---

## Текущее состояние

### Backend (Go 1.26, chi, pgx/v5)

| Bounded context | Файлы | Состояние |
|---|---|---|
| **auth** | `internal/auth/` | JWT, login flow, ratelimit на public-роуты |
| **leads** | `internal/leads/` | CRUD, AI-квалификация, drafts, identity-aggregation |
| **prospects** | `internal/prospects/` | CRUD, CSV import/export, dedup |
| **sequences** | `internal/sequences/` | Multi-step outreach, channel-aware steps, tracking pixel |
| **sources** | `internal/sources/` | Категории + источники + stats |
| **inbox** | `internal/inbox/` | Telegram bot + IMAP poller, attachments analyzer, HITL queue (pending replies) |
| **outbound** | `internal/outbound/` | Resend (primary), SMTP (fallback), MTProto (TG личный аккаунт) |
| **settings** | `internal/settings/` | Multi-tenant: per-user AI/SMTP/IMAP/Resend config + testers |
| **chat** | `internal/chat/` | AI-ассистент для оператора (изолирован от ai через адаптер) |
| **reminders** | `internal/reminders/` | Cron для stale leads |
| **verify** | `internal/verify/` | Email syntax/MX/SMTP-probe, TG-username проверка, disposable-домены |
| **parser** | `internal/parser/` | 2GIS, website scraping |
| **tgclient** | `internal/tgclient/` | MTProto (gotd/td) для отправки с личного TG |
| **audit** | `internal/audit/` | AsyncRecorder cost-tracking за каждый AI-call, decorator над ai.Provider |
| **ratelimit** | `internal/ratelimit/` | Redis-backed sliding window + in-memory fallback |
| **httputil** | `internal/httputil/` | JSON-response helpers + defence-in-depth body-size middleware |

### Frontend (Next.js 16, React 19)

| Страница | URL | Состояние |
|---|---|---|
| Login | `/login` | Реальный API |
| Inbox | `/inbox` | Реальный API, фильтры, статусы |
| Lead Detail | `/inbox/[leadId]` | Реальный API + drafts + identity-aggregation |
| Operator Queue | `/inbox/pending` | HITL queue с bulk decide, optimistic-remove, 10s polling |
| Prospects | `/prospects` | CRUD + CSV + verify integration |
| Sequences | `/sequences` | Multi-step builder, channel-per-step |
| Outbound | `/outbound` | Очередь отправки + tracking |
| Parser | `/parser` | 2GIS + website scraping |
| Settings | `/settings` | Sub-hooks per concern (AI/SMTP/IMAP/Resend/Telegram bot/account) |

### HITL (Human-In-The-Loop) кластер

Закрыт на 100% активной поверхности:

- **Inbound** — каждый AI-draft проходит approve-before-send через `pending_replies` table
- **Channel routing** — `channelReplyDispatcher` по `pr.Channel`, telegram + email parity
- **Operator queue page** `/inbox/pending` с bulk approve/reject
- **Email auto-draft** — booking-link suggestion на `DetectCallAgreement` match (email + telegram parity)
- **Защита от мисфайра** — empty-bookingLink config → suppress (не лендим пустой URL в queue)

### Инфраструктура

- docker-compose: PostgreSQL 18, Redis 8, Ollama (опционально)
- OrbStack на dev-машине
- Миграции: golang-migrate, 001-032 (audit_log, pending_replies, decided_by FK, и т.д.)
- Защита от DoS на body size: 10 MiB outer ceiling + 1 MiB JSON-specific cap (defence in depth)
- CLA bot, CI gates: Backend Go, Frontend Next.js, Redteam corpus, Tooling
- Release automation: `bin/release.sh X.Y.Z` синкает 4 version sync-points + tag + GH release

### Тесты

- Backend unit: ~350 тестов, ~66% coverage
- Backend integration (`go:build integration`): 55 тестов, ~79% с ними
- Frontend vitest: 115 тестов, ~43% coverage
- Domain packages: 100% coverage

---

## На горизонте (приоритет TBD)

### Outbound HITL
Inbound имеет full approve-before-send. Outbound (sequences) пока шлёт автоматически. Возможный mirror: each scheduled outbound message → `pending_replies` queue → operator approve. Trade-off: добавляет latency + manual surface vs. lower risk на cold outreach.

### Analytics dashboard
Outbound tracking (sequences pixel) уже собирает open events. Reply rate частично через identity-aggregation. **Нет**: UI-агрегации (open rate, reply rate, conversion rate per sequence, per user, per campaign). Audit-log с cost собирается, но UI отдельной cost-dashboard нет.

### Multi-workspace
Текущая модель: один владелец (`cfg.OwnerUserID`), single-tenant в продакшене с multi-tenant adapters внутри. Реальная multi-team разработка требует переосмысления — workspace как aggregate, RBAC, billing-per-workspace.

### Webhook integrations
Outgoing webhooks на ключевые события: `lead.created`, `pending_reply.approved`, `sequence.completed`. Для интеграции с CRM/Zapier/etc. без копирования данных.

### Auto-enrichment
По domain'у компании — обогащение из публичных источников (HH.ru, Rusprofile, открытые реестры). Без paid API сначала; платные интеграции (Clearbit, Apollo) — отдельный gate.

### Security follow-ups
- Per-route file-upload cap'ы (сейчас 10 MiB outer на importCSV; possibly tighter per-route)
- Audit-log retention/rotation (сейчас бесконечный grow)
- A/B тестирование промптов через `internal/ai` + audit-log group-by

### Observability foundations
- `/metrics` Prometheus endpoint
- Slow-query logging
- Уже есть structured slog, но без агрегации/алертинга

---

## Принципы

- **TDD + DDD + Clean Architecture** — механические гейты через `CLAUDE.md`
- **Bounded contexts** — каждый `internal/X` изолирован; cross-context только через адаптеры в `cmd/server/`
- **Domain-инварианты** только через фабрики (`NewLead`, `NewProspect`, `NewSource`...)
- **Без feat-коммитов с тестами одновременно** (TDD red→green pairs)
- **Версионирование** — semver, 4 sync-точки (VERSION, README badge, package.json, package-lock.json)
- **Без upмаунта в main** — только через PR + CI gate

---

## Архивированное (доделано, оставлено для истории)

Раздел "Сессия 1-5" предыдущей версии roadmap'а (verify, parser, мультиканальные секвенции, backend+frontend wiring, полировка) — полностью реализовано к маю 2026. Детали — в архитектурных хрониках `chronicles.md` (user-local).
