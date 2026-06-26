# Floq — Roadmap

> AI-помощник для полного цикла B2B-продаж: inbound (Telegram/Email) + outbound (cold outreach) + AI-квалификация и драфты.
> Обновлено: 2026-06-26

---

## Недавно сделано (v0.47.0–v0.57.0)

- **Auto-enrichment фаза 3 — юр.реквизиты компании через DaData (v0.57.0, #188):** новый порт `Enricher` (ортогонален `Extractor`: сам ходит во внешний реестр по идентичности) + адаптер `DaDataEnricher` в `cmd/server` поверх официального DaData API. После website-скрейпа подтягивает реквизиты (ИНН/ОГРН/адрес/ОКВЭД/офиц.название/статус) → секция «Реквизиты» в карточке. Матч честный: ИНН со страницы (regex с label-границей + checksum) → точный `findById`; иначе fuzzy `suggest` принимается ТОЛЬКО при единственном уникальном результате, иначе skip (ложные реквизиты хуже отсутствия). Domain: VO `INN`/`OGRN` с контрольными суммами (10/12 и 13/15 цифр), `LegalDetails` в `CompanyProfile` JSONB (backward-compat, без миграции). Best-effort: miss/ошибка/выкл — website-профиль цел (graceful degrade). Ship dark: `ENRICHMENT_REGISTRY_ENABLED` + `DADATA_API_KEY` (пустой ключ → выкл). Egress: фиксированный доверенный хост + ключ в заголовке (URL не из недоверенного ввода) → нет SSRF-поверхности; global rate-limit на квоту. Clean Arch: `enrichment` чист от DaData/http. Code-review (2 раунда, исправлен false-match при неоднозначном имени + добавлен rate-limit): TDD 9 / DDD 8 / CA 9 / Correctness 9 / Security 9. Backlog: Prometheus calls/hits counter для DaData.
- **Auto-enrichment фаза 2 — LLM-экстракция отрасли и размера компании (v0.56.0, #186):** за существующим портом `Extractor` добавлен `ChainExtractor` (OCP, usecase не тронут): детерминированный HTML-парсинг фазы 1 остаётся базой, а `LLMExtractor` аддитивно вытягивает `industry`/`company_size` из того же скрейпленного HTML — без нового внешнего источника. Ошибка LLM проглатывается (graceful degrade → отдаётся HTML-профиль). За флагом `ENRICHMENT_LLM_ENABLED` (default off, ship dark). Domain: typed enum `CompanySize` + `ParseCompanySize`-инвариант, нормализатор `Industry`, оба в `CompanyProfile` JSONB (backward-compat, без миграции для полей). Clean Arch: `enrichment` не импортит `internal/ai`/`audit` — LLM за локальным портом `Completer`, склейка (cost-cap + audit CallMeta) в адаптере `cmd/server`. Cost-cap: Budget-mode + MaxTokens + input rune-cap + per-domain rate-limit; cost-audit под `request_type='enrichment'` (миграция 049 расширяет CHECK `audit_log`). UI: чипы отрасль/размер в карточке «О компании». **Code-review поймал CRITICAL: `WithRequestType` — no-op без родительской CallMeta → LLM-вызовы фонового воркера были un-audited; фикс — сквозная subject-user атрибуция + свежая `ContextWithCallMeta`.** Раунд 2: TDD 9 / DDD 9 / CA 9 / Correctness 9 / Security 9. Следующее — фаза 3 (registry-источники за портом `Enricher`).
- **Auto-enrichment лидов по домену компании — фаза 1 (v0.55.0, #182):** при создании лида/проспекта с корп-email фоновый воркер (`EnrichmentCron`) скрейпит сайт компании (по домену из email) и тянет профиль (title/description/контакты/соцсети) чистым HTML-парсингом (без LLM). Новый bounded context `internal/enrichment` (domain VO `Domain`/`Status`/`CompanyProfile` + порты Store/PageFetcher/Extractor/RateLimiter, DIP). Таблица-очередь `company_enrichment` (per-user, миграция 048) = кэш+дедуп+retry; enqueue best-effort из prospects+inbox через узкие порты. Read `GET /api/enrichment?email=` (tenant-scoped) → карточка «О компании» на лиде. **SSRF-защита в 2 слоя** (VO режет IP/host:port + egress-guarded клиент по resolved IP, бьёт rebinding/redirect). Порт `Extractor` — шов под LLM в фазе 2. Code-review: TDD 8 / DDD 9 / CA 9 / Security 8 / Correctness 8 (после фикса critical SSRF + мёртвого retry).
- **Ротация KEK для секретов at-rest (v0.54.0, #178):** primary + secondary fallback без даунтайма и без потери данных. `FLOQ_SECRETS_KEK` (primary: шифрование + первая попытка decrypt) + опциональный `FLOQ_SECRETS_KEK_OLD` (decrypt-only fallback на время ротации). Команда `server -rotate-secrets` перешифровывает все секреты под primary (convergent, безопасно повторять; шифротекст без key-id); read-only `server -verify-secrets-kek` доказывает завершённость (primary-only decrypt, exit non-zero пока есть un-rotated — гейт на удаление KEK_OLD). Без миграции (048 свободна). Runbook `docs/kek-rotation.md`. TDD: 4 RED→GREEN пары + pin-тест набора колонок + unit-тест exit-гейта; code-review TDD/DDD/CA = 8/9/9.
- **Secrets-at-rest: drop plaintext-колонок (v0.53.0, #175):** миграция 047 удаляет legacy plaintext-колонки секретов (`user_settings` ×5 + `onec_credentials.auth_secret`) — чтение/запись только через `*_enc`/`*_nonce` (AES-256-GCM), plaintext-fallback убран. SQL-guard в 047 (RAISE если есть un-backfilled секрет) защищает от потери данных; backfill через `server -backfill-secrets` (запуск ДО миграций, exit); recovery задокументирован (golang-migrate CLI `force 46`). Изолированный migration-тест пинит все 6 предикатов guard'а + idempotency/empty-skip/round-trip. 3 раунда code-review (recovery-able guard, разворот авто→explicit backfill, per-column coverage). Остаётся: ротация KEK (#178).
- **Экран архивных лидов + разархивирование (v0.52.0, #174):** отдельная страница `/inbox/archived` (ссылка «Архив» в шапке ленты) показывает только заархивированные лиды (новые сверху) с разархивированием в один клик; на детальной странице кнопка «Архив» переключается на «Разархивировать» по `archived_at`. Бэкенд: `GET /api/leads/archived` (`ListArchivedLeads` — `WHERE archived_at IS NOT NULL ORDER BY archived_at DESC`), `archived_at` в `LeadResponse` (omitempty), partial index (миграция 046). TDD RED→GREEN по слоям; code-review (high, workflow) — исправлены все находки: load-error vs пустой архив, per-row guard разархива, «Только что назад», общий `unarchiveLead`-хелпер + `LeadAvatar`, дедуп repo-скана. Закрывает follow-up #174 от #173.
- **Настоящий архив лида (v0.51.0, #173):** флаг `archived_at` **ортогонален** pipeline-статусу (НЕ `status='closed'` — closed терминальный и значим для воронки). Архив скрывает лид из **рабочих лент + current-pipeline** (лента, hot-leads, inbox ByStatus/qual-dist, suggestion/usage/source/chat counts, напоминания — через SQL-view `active_leads`), но **НЕ из финансовой/операционной истории и actionable HITL-очереди** (cost-ratios, pending-stats, очередь ответов — по base-таблицам). Авто-разархив при ре-энгейджменте (входящее TG/email + CSV re-import). CSV-экспорт/импорт round-trip'ит точный `archived_at` (+ фикс BOM, из-за которого свой экспорт не реимпортировался). Two-step confirm перед архивом; guarded `SetLeadArchived` (идемпотентность → 409). Миграции 044 (колонка + matview) + 045 (`active_leads` view). 3 раунда code-review (10→10→6, сходимость). Follow-up: #174 (экран архива + unarchive UI), #175 (Secrets-at-rest).
- **Фильтр напоминаний по срочности (v0.50.0, #170):** в шапке «Напоминаний» живой клиентский фильтр «Все срочности / Критичные / Предупреждения» (порог тот же `≥4 дня молчания`, что и в сводке) вместо мёртвой кнопки; счётчики/сводка остаются по всем фоллоуапам. Мёртвая кнопка «Очистить все» убрана (нет бэкенда под массовый dismiss). Доступный `<select>` с focus-индикатором. Tech-debt из ux-clarity «нет мёртвого UI».
- **Broader sequences-IDOR (v0.49.2, #162/#163):** все id-адресуемые операции контекста `sequences` скоупятся по владельцу — sequence get/update/delete/toggle, add/delete step и launch (owner via `Sequence.UserID`), outbound approve/reject/edit (owner via `prospect.UserID`). Отсутствующий и чужой ресурс → один sentinel (`ErrSequenceNotOwned`/`ErrMessageNotOwned`) → **404** (анти-enumeration). Авторизация в usecase, handler только маппит; добавлен порт `GetStep`. Хардненинг: JWT-middleware отклоняет nil-UUID. Закрывает остаток от #160.
- **IDOR-фикс запуска последовательности (v0.49.1, #154/#160):** `Launch` теперь требует authenticated userID и отклоняет любой проспект, не принадлежащий вызывающему (→ 404, без cross-tenant enumeration). Раньше можно было запустить рассылку на чужих проспектах; после автопилота это означало реальную отправку чужим. Под транзакцией частичный батч откатывается.
- **Period-окна воронки (v0.49.0, #158):** на странице «Конверсия» — выбор периода (Неделя/Месяц/Всё время). Миграция 043 re-grain'ит funnel-матвью на аддитивную грануляцию (qual-distribution по дню; conversion дедупится до (sequence, step, prospect) + `entered_at`), оконный счёт точен без `COUNT(DISTINCT)`-аддитивности; `NOW()` только в read-query.
- **Автопилот (v0.47.0, #153):** флаг `auto_send` подключён к пути отправки — при включении сообщения последовательности авто-аппрувятся при launch и уходят фоновым sender'ом с grace-window (`auto_send_delay_min`). Default OFF; единый контрол на «Автоматизациях», read-only статус на «Очереди отправки». Fail-safe + инвариант одного владельца на launch. Известное: IDOR в launch вынесен в #154.
- **Ясность навигации (v0.48.0, #155):** `/alerts` «Лиды» → «Напоминания» (реальная лента лидов — «Входящие»); в аналитике «Воронка» → «Конверсия» (Pipeline остаётся «Воронка») — убрана коллизия названий.
- **e2e Playwright (#156):** первый e2e-слой (мок бэкенда через `page.route`, 8 журнеев: auth / навигация / автопилот / approve). Матрица покрытия **unit + integration + e2e — 6/6**. Запуск `npm run test:e2e`.

---

## Текущее состояние

### Backend (Go 1.26, chi, pgx/v5)

| Bounded context | Файлы | Состояние |
|---|---|---|
| **auth** | `internal/auth/` | JWT, login flow, ratelimit на public-роуты |
| **leads** | `internal/leads/` | CRUD, AI-квалификация, drafts, identity-aggregation |
| **prospects** | `internal/prospects/` | CRUD, CSV import/export, dedup; consent VO {none/obtained/withdrawn} + suppression-список + подписанная отписка (HMAC) — compliance core (v0.43.0); получение согласия — CSV-колонка / ручной toggle (UI-бейдж) / авто при inbound-ответе, withdrawn нерушим (v0.44.0) |
| **sequences** | `internal/sequences/` | Multi-step outreach, channel-aware steps, tracking pixel |
| **sources** | `internal/sources/` | Категории + источники + stats |
| **inbox** | `internal/inbox/` | Telegram bot + IMAP poller, attachments analyzer, HITL queue (pending replies) |
| **outbound** | `internal/outbound/` | Resend (primary), SMTP (fallback), MTProto (TG личный аккаунт); send-gate suppression→consent (fail-closed), List-Unsubscribe заголовки (RFC 8058) (v0.43.0) |
| **settings** | `internal/settings/` | Multi-tenant: per-user AI/SMTP/IMAP/Resend config + testers; секреты шифруются at-rest (AES-256-GCM) |
| **secrets** | `internal/secrets/` | AES-256-GCM SecretCipher (KEK из `FLOQ_SECRETS_KEK`), at-rest шифрование клиентских учёток; fail-fast при невалидном ключе (v0.42.0) |
| **chat** | `internal/chat/` | AI-ассистент для оператора (изолирован от ai через адаптер) |
| **reminders** | `internal/reminders/` | Cron для stale leads |
| **verify** | `internal/verify/` | Email syntax/MX/SMTP-probe, TG-username проверка, disposable-домены |
| **parser** | `internal/parser/` | 2GIS, website scraping |
| **tgclient** | `internal/tgclient/` | MTProto (gotd/td) для отправки с личного TG |
| **audit** | `internal/audit/` | AsyncRecorder cost-tracking за каждый AI-call, decorator над ai.Provider |
| **analytics** | `internal/analytics/` | Read-side projections (sequence performance, cost ratios, hot leads) — DTO-only |
| **ratelimit** | `internal/ratelimit/` | Redis-backed sliding window + in-memory fallback; per-IP auth-route limits |
| **metrics** | `internal/metrics/` | Prometheus `/metrics`: HTTP + AI-cost + audit-drops + queue-depth + runtime |
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
| Analytics | `/analytics/sequences`, `/analytics/cost`, `/analytics/inbox`, `/analytics/hot-leads` | Дашборд из 4 view'ов: sequence performance + cost + inbox flow + hot-leads (v0.28.0 → inbox-flow v0.40.0; эпик #91 закрыт) |
| Integrations | `/settings` → секция 1С | Двусторонняя интеграция с 1С: webhook-приём + outbound OData + маппинг + reconcile + UI настроек/секретов (эпик #105, v0.41.0) |
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
- Миграции: golang-migrate, 001-040 (audit_log, pending_replies, decided_by FK, onec_credentials/mapping, audit_log_daily retention, encrypt_secrets at-rest, 038 prospect_consent, 039 suppressions, 040 pending_replies.input_severity, и т.д.; 40 файлов .up.sql). Drop plaintext-колонок секретов — отдельной миграцией (следующий свободный номер) после верификации бэкфилла на проде
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

### Analytics dashboard (shipped — эпик #91 закрыт)
`/analytics/sequences` (#95) — per-sequence sent/delivered/opened/replied/converted с rates. `/analytics/cost` (#96) — total AI-cost + cost-per-{lead,qualified,converted,draft} + by-request-type/by-model breakdowns. `/analytics/inbox` (#97, v0.40.0) — inbox flow: leads by channel/status + score-гистограмма + pending-replies approve-rate + p50/p95 time-to-decide. `/analytics/hot-leads` (#98, v0.39.0) — лиды по убыванию скора квалификации, фильтры status/channel/period. Все 4 view готовы.

### Multi-workspace
Текущая модель: один владелец (`cfg.OwnerUserID`), single-tenant в продакшене с multi-tenant adapters внутри. Реальная multi-team разработка требует переосмысления — workspace как aggregate, RBAC, billing-per-workspace.

### Webhook integrations
Outgoing webhooks на ключевые события: `lead.created`, `pending_reply.approved`, `sequence.completed`. Для интеграции с CRM/Zapier/etc. без копирования данных.

### Auto-enrichment
По domain'у компании — обогащение из публичных источников (HH.ru, Rusprofile, открытые реестры). Без paid API сначала; платные интеграции (Clearbit, Apollo) — отдельный gate.

### Security follow-ups
- **Agent-security guardrails — пилот (v0.45.0):** 4 слоя + PII в `internal/ai/security`, подключены на пути inbox→LLM (декоратор `guardedQualifier`) и outbound (`SendGuard` порт). Слой 1 inputFirewall (инъекции), 1b PIIScrubber (обратимый), 2 OutputValidator (clamp/redact/confidence), 3 OutboundGuard (канал/получатель/mass-send), 4 CostBreaker (cap+budget). Red-team корпус 38, CI-гейт. Threat-model `docs/security-model.md` v1.1 (MITRE ATLAS+OWASP LLM).
- **L2 tool-call firewall reply-path — сделано (v0.46.0):** `ToolCallFirewall` подключён в путь отправки HITL-ответов. Входящее сообщение классифицируется при `Propose` (порт `inbox.InputClassifier` над `security.InputFirewall`), severity хранится в `pending_replies.input_severity` (миграция 040, grandfather-дефолт `info`), декоратор `guardedReplyDispatcher` гейтит отправку: Block → отказ даже после approval, Warn/Info → отправка. Reply-диспетчеры вынесены в `internal/inbox` через порт `ReplyTargetLookup` (рефактор, PR #138). **Остаётся:** промоушен стандарта agent-security в `active` после ≥4 нед live-метрик (ASR пока structural на фикстурах, не live).
- **At-rest шифрование секретов клиента — сделано (v0.42.0):** AES-256-GCM, KEK из env, миграция 037 (enc/nonce-колонки) + идемпотентный бэкфилл (`server -backfill-secrets`). **Drop plaintext-колонок — сделано (#175, миграция 047):** чтение/запись только через ciphertext; guard в 047 (RAISE если есть un-backfilled секрет — защита от потери данных) с задокументированным recovery (golang-migrate CLI `force 46` → `-backfill-secrets` → restart). **Ротация KEK — сделано (#178, v0.54.0):** primary + secondary fallback (`FLOQ_SECRETS_KEK` + опц. `FLOQ_SECRETS_KEK_OLD`), команды `server -rotate-secrets` (convergent re-encrypt) + `server -verify-secrets-kek` (read-only гейт на удаление старого KEK), runbook `docs/kek-rotation.md`. Остаётся: `webhook_secret` хеширование
- `webhook_secret` 1С — пока plaintext lookup-токен; хеширование (не шифрование, ломает lookup) — отдельная задача
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
