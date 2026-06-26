# Integration audit — Floq pre-MVP launch

**Дата:** 2026-05-15
**Версия кода:** v0.8.2 (commit `ed97fae`, ветка `main`)
**Скоуп:** интеграционный слой Floq (PostgreSQL + Redis + IMAP + Telegram + 2GIS + 4 LLM-провайдера) против 4 требований к AI-агентам, опубликованных Диасофт (Digital Q.Integration кейс).
**Источник методологии:** Диасофт habr.com/ru/companies/diasoft_company/articles/1034996/, KB `agent-security-defaults v1.0`, `reflective-agent-defaults v1.4`.

---

## 1. Сводка вердиктов

| # | Требование | Вердикт | P-блок |
|---|---|---|---|
| 1 | Время отклика ≤500ms | **PARTIAL** (ack-путь PASS, end-to-end с LLM FAIL) | P1 |
| 2 | Семантическая нормализация | **FAIL** | **P0** |
| 3 | Двусторонность с аудитом | **FAIL** (нет audit_log, нет idempotency, частичный HITL) | **P0** |
| 4 | Гарантированная доставка | **PARTIAL** (есть наивный retry через тикер, нет DLQ/backoff/exactly-once) | P1 |

**Executive summary.** Codebase архитектурно зрелый (Clean Architecture + DDD + 100% domain coverage), но **два требования из четырёх блокируют MVP launch для enterprise-аудитории**: отсутствует audit_log таблица как таковая, и каждый bounded-context имеет собственную копию `Lead` без сквозной нормализации полей (email/phone/username). До MVP launch — закрыть P0 (требования 2 и 3) за 3-5 рабочих дней. P1 (retry/DLQ/backoff, end-to-end latency для LLM через Redis-кэш и async-pipeline) — в первые 2 недели после launch.

---

## 2. Требование 1 — Время отклика

### Что проверял
- AI-квалификация одного лида end-to-end (входящий Email/TG → готовая qualification) ≤500ms
- Кэширование часто-запрашиваемых данных (контекст клиента, провайдер, prompt-templates) в Redis
- N+1 query patterns / eager loading
- Connection pool под параллельные запросы
- Async/blocking обработка LLM-вызова

### Найдено

| Аспект | Статус | Файл:строка | Деталь |
|---|---|---|---|
| Telegram inbound ack | PASS | `internal/inbox/telegram.go:63-188` | 4-5 sequential DB-запросов до ack, без LLM в критическом пути → реально <100ms на локальной БД |
| Email IMAP latency | UNCLEAR | `internal/inbox/email.go:54` | polling каждые 60 сек — не push-режим. Худший случай: лид виден через 60 сек после поступления |
| LLM в критическом пути | PASS (async) | `internal/inbox/telegram.go:154-187`, `email.go:329-361` | Qualify запускается в `go func()` с timeout 30 сек — пользователь не ждёт LLM |
| End-to-end qualification ≤500ms | **FAIL** | `internal/ai/client.go:86-111` | Анализ через Claude/GPT-4o/Ollama — типично 1-5 сек, иногда до 10 сек. ≤500ms физически недостижимо для cloud LLM |
| Redis кэширование | **FAIL** | `internal/config/config.go:50` | `RedisURL` в конфиге, но `redis.Client` нигде не инстанцируется (`grep "redis\." ./internal -r` пусто). Контекст клиента, prompt-template, предыдущие qualifications — каждый раз заново |
| pgxpool config | **PARTIAL** | `cmd/server/main.go:59` | `pgxpool.New(ctx, url)` без явного `pgxpool.Config{MaxConns, MinConns, HealthCheckPeriod}`. Получаем дефолт `4 * runtime.NumCPU` — на 8-core это 32 соединения. Для single-tenant MVP норм, но не задокументировано как осознанный выбор |
| N+1 при IMAP poll | **PARTIAL** | `internal/inbox/email.go:267-327` | На каждое письмо: `GetLeadByEmailAddress` + `FindByEmail` (prospect lookup) + `CreateLead` + `ConvertToLead` + `MarkRepliedByProspect` + `CreateMessage`. Минимум 4-6 round-trip per email. На 100+ непрочитанных письмах — заметно |
| Async goroutine context | **FAIL** | `internal/inbox/telegram.go:157`, `email.go:330` | `context.Background()` для qualification — НЕ родительский ctx. При graceful shutdown сервер уйдёт, а goroutine продолжит на 30 сек без отмены |

### Вердикт
**PARTIAL.** Ack-путь до пользователя укладывается в 500ms — это правильная архитектура (LLM async). Но строгая формулировка Диасофт «AI-квалификация end-to-end ≤500ms» не выполнима принципиально для cloud LLM. Нужно либо:
- (a) переформулировать SLO для пользователя на «ack ≤500ms, qualification eventual ≤30s», либо
- (b) для критичных лидов (по lookup в `prospects` с высоким priority) — locally-cached classification через Redis + lightweight model (Ollama haiku-class).

Redis отсутствует как working dependency — это **архитектурный долг**, а не блокер launch.

---

## 3. Требование 2 — Семантическая нормализация

### Что проверял
- Единая модель Lead/Contact/Deal в `domain/`
- Mapping IDs между источниками (один контакт через IMAP и TG = один Lead?)
- Поля даты/телефона/email — единый формат (RFC 3339 / E.164 / lowercase)?
- Industry/CompanySize — typed enum или magic strings?
- Normalization layer перед записью при парсинге 2GIS

### Найдено

| Аспект | Статус | Файл:строка | Деталь |
|---|---|---|---|
| Один конструктор `NewLead` | **FAIL** | `internal/leads/domain/entity.go:105` + `internal/inbox/ports.go:40` + `internal/inbox/helpers.go:NewInboxLead` | Существуют **минимум два разных типа Lead**: `domain.Lead` (для leads-контекста) и `inbox.InboxLead` (для inbox-контекста). Это намеренное cross-context isolation per Clean Architecture, **но семантическая нормализация размазана**: каждый контекст имеет собственный конструктор и собственные правила валидации |
| Identity resolver IMAP↔TG | **FAIL** | `internal/inbox/telegram.go:91`, `internal/inbox/email.go:268` | TG-входящее ищется по `chat_id`, email-входящее — по `email`. **Один человек, написавший в TG @vladimir и потом приславший письмо vladimir@example.com — два разных лида.** `LeadChecker` адаптер существует только для prospects↔leads дедупа, не для cross-source identity. (Issue #27 уже заведён.) |
| Email lowercase в storage | **FAIL** | `internal/leads/domain/entity.go:122`, `internal/prospects/domain/entity.go:131` | `NewLead` и `NewProspect` принимают email как `string` без `strings.ToLower(strings.TrimSpace(...))`. `shouldSkipEmail` (`internal/inbox/email.go:214`) делает ToLower на check-time, но при записи `from.Addr()` идёт как есть. `john@x.com` и `John@x.com` создадут разных лидов |
| Phone E.164 | **FAIL** | `internal/prospects/domain/entity.go:96` | `Prospect.Phone string` — без normalization. CSV-импорт сохраняет как пришло. `+7 (916) 123-45-67`, `89161234567`, `7-916-123-45-67` — три разных prospect'а |
| TG username нормализация | **FAIL** | `internal/inbox/telegram.go:89`, `internal/prospects/domain/entity.go:98` | `username := msg.From.UserName` — без strip `@` и без ToLower. Lookup `FindByTelegramUsername(... username)` — точное совпадение с тем, что лежит в БД. CSV-импорт `@Vladimir` против runtime `vladimir` → два разных проспекта |
| Industry/CompanySize typed | **FAIL** | `internal/prospects/domain/entity.go:99-100` | `Industry string`, `CompanySize string` — magic-strings. Нет `type Industry string; const (...)`. Невозможно безопасно фильтровать «IT-компании 50-200 человек» |
| Дата формат | PASS | `internal/leads/domain/entity.go:112`, `usecase.go:216` | `time.Now().UTC()` везде, CSV-export через `time.RFC3339` |
| 2GIS normalization layer | **PARTIAL** | `internal/parser/twogis.go:62` | `cityLower := strings.ToLower(city)` есть для запроса. Но нет structured-extraction для phone/email/website перед записью в `Prospect` |

### Вердикт
**FAIL.** Acceptance из задачи: «PASS если ОДИН конструктор `NewLead(...)` в domain с валидацией + все источники проходят через него». Реальность:
- Два конструктора (`leads.NewLead`, `inbox.NewInboxLead`) — это требование DDD bounded contexts
- Но при этом **нормализация полей (email/phone/username) отсутствует в обоих**
- Identity resolver между источниками отсутствует совсем

Это **блокирует enterprise launch**: первый же демо-кейс «вот клиент, он в TG и Email» покажет два разных лида в инбоксе.

---

## 4. Требование 3 — Двусторонность с аудитом

### Что проверял
- При write-операциях во внешние системы (отправка письма, TG, CRM update) — лог в audit_log?
- Idempotency_key для повторных вызовов
- Откат write-операций (compensating transaction)
- Контроль прав `has_permission(user, action)` перед execute
- AI-quality call (агент решил отправить) — HITL или auto-fire?

### Найдено

| Аспект | Статус | Файл:строка | Деталь |
|---|---|---|---|
| Таблица `audit_log` | **FAIL** | `backend/migrations/` (001-024) | **Таблицы не существует.** `grep audit migrations/` пусто. Все write-операции (отправка письма, TG-сообщения, обновление prospect.verify_status, статусы лидов) логируются только через `log.Printf` в stdout — никакой структурированной истории |
| Idempotency_key | **FAIL** | `internal/outbound/sender.go:96-167` | `OutboundMessage` идентифицируется через `msg.ID` (UUID), но **нет idempotency_key в API call** к Resend/SMTP. Если SMTP вернул 200 но `seqRepo.MarkSent` упал (DB hiccup) — следующий тик через 30 сек ре-отправит письмо. Дубликат гарантирован |
| Compensating transaction | **PARTIAL** | `internal/outbound/sender.go:133-148` | Bounce → `MarkBounced` + `UpdateVerification(prospect, invalid)` — это compensating-семантика. Но не обёрнуто в транзакцию: если первое прошло, второе упало — рассогласование |
| Permission check | **PARTIAL** | `cmd/server/main.go:147-152` | JWT auth-middleware проверяет identity. Все handler'ы scoped по `userID := httputil.UserIDFromContext(...)`. Но **нет per-action permission**: «может ли user X отправить письмо клиенту Y?» проверяется только через ownership prospect'а, не через явную permission-grid |
| HITL для драфтов | PASS | `internal/leads/usecase.go:149-186` | `RegenerateDraft` создаёт draft → пользователь видит в UI → нажимает «отправить». Драфт не fires автоматически |
| HITL для outbound queue | PASS | `internal/outbound/sender.go:91` | `GetPendingSends` отбирает только `status='approved'`. Approval идёт через UI явным действием пользователя |
| HITL для AI sequence step | PASS | `internal/settings/usecase.go:32` | `AutoSend bool` setting — **по умолчанию false**. Каждый шаг последовательности требует approval, если auto_send выключен |
| **AUTO-FIRE: booking link на TG** | **FAIL** | `internal/inbox/telegram.go:140-151` | `DetectCallAgreement(text)` — regex на «давайте созвонимся» — **автоматически отправляет booking link** клиенту без подтверждения. Если regex срабатывает на иронию/негатив → клиент получает «отлично, вот ссылка!» в неудобный момент. Нет confirmation step |
| Out-of-band confirmation на destructive | **FAIL** | весь outbound | Отправка письма / TG / mark-bounced — никакого out-of-band подтверждения. `auto_send=true` = полностью автономно |

### Вердикт
**FAIL.** Отсутствие `audit_log` — это первое, что спросит enterprise-клиент при безопасности-ревью. Idempotency отсутствие = риск двойной рассылки клиенту при flaky DB. Auto-fire booking link на regex = риск UX-инцидента.

---

## 5. Требование 4 — Гарантированная доставка

### Что проверял
- Retry policy (exponential backoff + jitter) для transient failures
- Dead letter queue для failed messages
- At-least-once с подтверждением получателя
- Exactly-once для критичных операций (unique transaction_id)
- Мониторинг очередей (алерт если DLQ > N)

### Найдено

| Аспект | Статус | Файл:строка | Деталь |
|---|---|---|---|
| Retry на transient failures | **PARTIAL** | `internal/outbound/sender.go:148-150` | Не-bounce ошибки → `log.Printf + continue` → следующий тик через 30 сек подберёт сообщение снова. Это **наивный fixed-interval retry**, без exponential backoff, без jitter, без max-attempts |
| Bounce detection | PASS | `internal/outbound/sender.go:135` | Распознаёт ключевые слова (`bounce`, `invalid`, `rejected`, `mailbox`, `550`, `553`) и mark'ает invalid — не зацикливается на dead-адресах |
| Dead Letter Queue | **FAIL** | весь outbound | DLQ-таблицы нет. Сообщение в `outbound_messages` либо `sent`, либо вечно `approved` — будет ретраиться каждый 30s до конца времён |
| At-least-once с ack получателя | **FAIL** | `internal/outbound/sender.go:399-410` | Resend/SMTP HTTP 200 = «принято к доставке», не = «доставлено». Tracking pixel (`/api/track/open/<id>`) есть для opens, но ack-loop на delivered/bounced webhook не настроен |
| Exactly-once | **FAIL** | см. выше: idempotency_key отсутствует | При concurrency-проблеме / повторе тика возможна дубль-отправка |
| Мониторинг очередей | **FAIL** | весь codebase | `prometheus`, `metrics`, `/metrics` endpoint — нет. Алертинг невозможен. Логи только в stdout |
| TG rate limit | PASS | `internal/outbound/sender.go:288-304` | `tgRateInterval = 90 * time.Second` — protection от flood. Корректный mutex |
| MTProto retry/timeout | PASS | `internal/outbound/sender.go:338-355` | Per-target loop (username, потом phone) с 60s timeout — есть sane fallback |
| IMAP poll error handling | **PARTIAL** | `internal/inbox/email.go:91-127` | Любая ошибка → `log.Printf + return` → следующий тикер через 60 сек. Effectively retry, но не различает transient (network) от permanent (auth fail) |

### Вердикт
**PARTIAL.** Базовая «retry through tick» работает для outbound. Но enterprise-критичные требования — DLQ, exponential backoff, exactly-once, метрики — отсутствуют.

---

## 6. Приоритизация P0 / P1 / P2

### P0 — БЛОКИРУЮТ MVP LAUNCH

| # | Что | Эстимейт | Связано |
|---|---|---|---|
| P0-1 | **`audit_log` таблица + middleware** для всех write-операций (send_email, send_tg, update_prospect_verify, transition lead status, approve_step) | 1.5 дня | Требование 3 |
| P0-2 | **Email/TG-username/phone normalization в domain factories** (`NewLead`, `NewProspect`, `NewInboxLead`): ToLower email + trim, strip `@` + ToLower username, E.164 phone | 0.5 дня | Требование 2 |
| P0-3 | **HITL для booking-link auto-reply**: вместо мгновенной отправки — создать draft + push-notification менеджеру, требовать подтверждения. Либо: `auto_reply_booking` setting (default false) | 0.5 дня | Требование 3 |
| P0-4 | **Idempotency-key для outbound send**: использовать `msg.ID` как HTTP-header `Idempotency-Key` для Resend, для SMTP — обернуть `MarkSent + send` в БД-транзакцию с `SELECT FOR UPDATE SKIP LOCKED` | 1 день | Требование 3 + 4 |
| P0-5 | **Использовать parent ctx в async qualification goroutines** (заменить `context.Background()` на child от `ctx`) | 0.25 дня | Требование 1 (graceful shutdown), частично 4 |

**Итого P0: 3.75 дня.** Реалистично 4-5 дней с тестами по TDD-дисциплине.

### P1 — ЗАКРЫТЬ В ПЕРВЫЕ 2 НЕДЕЛИ ПОСЛЕ LAUNCH

| # | Что | Эстимейт |
|---|---|---|
| P1-1 | **Identity resolver** (issue #27) — Phase 2 multi-source aggregation. Email + phone + tg_username matchers | 3-4 дня |
| P1-2 | **Exponential backoff + jitter + max-attempts** для outbound. После N=5 попыток → DLQ-flag (`status='failed'`) | 1 день |
| P1-3 | **DLQ view + dashboard** — отдельный endpoint `GET /api/outbound/dlq`, frontend page | 1 день |
| P1-4 | **Resend webhook handler** — `POST /api/webhooks/resend` для events `email.delivered`, `email.bounced`, `email.complained` → закрыть at-least-once-with-ack | 1 день |
| P1-5 | **Typed enums Industry / CompanySize** (как было сделано для VerifyStatus в issue #21) | 0.5 дня |
| P1-6 | **Redis-кэш для prospect-context lookup** — горячие prospect'ы при входящем письме/TG читать из Redis (TTL 5 мин), invalidate на write | 1.5 дня |

**Итого P1: 8-9 дней.**

### P2 — ПОСЛЕ СТАБИЛИЗАЦИИ

| # | Что | Эстимейт |
|---|---|---|
| P2-1 | **Exactly-once для критичных операций** через `transaction_id` + outbox pattern | 3 дня |
| P2-2 | **Prometheus метрики** (`/metrics`) + Grafana dashboard (queue depth, send rate, error rate) | 2 дня |
| P2-3 | **MCP-server exposure** — выставить Floq как MCP-tool для AI-агентов сторонних клиентов | 5+ дней |
| P2-4 | **Bulk processing** для IMAP poll (batch lookups вместо N+1 round-trips) | 1 день |
| P2-5 | **pgxpool tuning** — explicit `MaxConns/MinConns/HealthCheckPeriod`, документированный SLO под нагрузку | 0.5 дня |

---

## 7. Acceptance критерии после фиксов

После применения P0:
- [ ] Миграция `025_create_audit_log.up.sql` создана; таблица содержит `id, user_id, action, resource_type, resource_id, args_json, result_json, created_at, correlation_id`
- [ ] Middleware/decorator вокруг каждого outbound-action пишет в audit_log
- [ ] `NewLead`, `NewProspect`, `NewInboxLead` нормализуют email (`strings.ToLower(strings.TrimSpace(...))`), tg_username (strip leading `@`, ToLower), phone (через `nyaruka/phonenumbers` E.164)
- [ ] Существующие тесты проходят. Добавлены unit-тесты на нормализацию (table-driven, ≥3 кейса каждое поле)
- [ ] Booking-link auto-reply: либо отключён по умолчанию (setting), либо создаёт draft вместо мгновенной отправки
- [ ] Outbound send использует `Idempotency-Key: <msg.ID>` для Resend
- [ ] Async LLM qualification использует child context от parent (тестируется shutdown-сценарием)
- [ ] Independent code-review агент по этому документу выставляет «P0 closed: 5/5»

После P1:
- [ ] Identity resolver мержит дубликаты по email/phone/tg_username
- [ ] DLQ-view с failed messages
- [ ] Resend webhook закрывает at-least-once
- [ ] Redis-кэш prospect-context работает с invalidate-on-write

---

## 8. Что НЕ покрыто этим аудитом

- Frontend integration (CORS, auth-flow, XSS) — отдельный security-review
- Сами LLM-промпты на prompt-injection — закрывается P1 security pilot (`agent-security-defaults v1.0`)
- Стоимость per request на разных провайдерах — закрывается ModelMode mapping (см. `docs/ai-model-selection.md`)
- Нагрузочное тестирование (k6/locust) — после P1 фиксов
- Тестовое покрытие репозиториев (`*Repository`) — частично через `-tags=integration`

---

## 9. Источники и методология

- Диасофт Digital Q.Integration кейс — habr.com/ru/companies/diasoft_company/articles/1034996/
- KB стандарт `agent-security-defaults v1.0` (status: draft → pilot)
- KB стандарт `reflective-agent-defaults v1.4` Правило 4 (Infrastructure-side enforcement)
- Gartner 2026: «50% AI-инициатив не доживут до прода» — основа аргумента «audit до launch дешевле refactor после»

---

**Аудит проведён:** read-only анализ кода `main@ed97fae`. Никаких изменений в коде в рамках этого документа не выполнено. Каждый P0-fix — отдельный PR с TDD-парами.
