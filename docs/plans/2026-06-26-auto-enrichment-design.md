# Auto-enrichment design (#182) — Phase 1

**Date:** 2026-06-26 · **Target release:** v0.55.0 · **Migration:** 048 (next free)

## Context

Лиды/проспекты приходят с минимумом данных (имя, канал, `Company`-строка, email). Нет автоматического обогащения. Цель — по домену компании автоматически подтянуть публичные данные для лучшей квалификации/таргетинга, показать на карточке.

Решения зафиксированы на brainstorming (5 развилок):
1. **Источник фазы 1:** website-скрейпинг **собственного сайта компании** (домен из email). Переиспользует `internal/parser`, без ToS/anti-bot рисков чужих сервисов. Rusprofile/ЕГРЮЛ/HH.ru — фаза 2 за тем же портом.
2. **Хранилище:** отдельная таблица `company_enrichment`, ключ по домену → естественный кэш/дедуп.
3. **Триггер:** таблица-как-очередь + cron-воркер (durable, retry, rate-limit, переживает рестарт).
4. **Scope:** **per-user** `(user_id, domain)` — консистентно с tenant-isolation, CASCADE по users.
5. **Извлечение:** **гибрид** — фаза 1 шипит только `HTMLExtractor` (чистый HTML, без LLM, детерминированно), но вводим порт `Extractor` как шов под `LLMExtractor` в фазе 2 (без audit-cost/недетерминизма сейчас).

## Архитектура — новый bounded context `internal/enrichment` (DDD + Clean Arch)

### domain/
- Entity `CompanyEnrichment`: userID, Domain, Status, Profile, Error, Attempts, EnrichedAt, ExpiresAt. Конструктор `NewPendingEnrichment(userID, domain)`; методы `MarkEnriched(profile)`, `MarkFailed(reason)` с валидацией переходов.
- VO `Domain`: `NewDomain(email) (Domain, error)` — нормализация (lowercase, срез `www.`), валидация, **отсев free-провайдеров** (gmail/yandex/mail.ru/outlook/hotmail/icloud/proton/…). Доменные ошибки `var ErrFreeEmailProvider`, `var ErrInvalidDomain`.
- VO `Status` (typed enum): `pending`/`enriched`/`failed` с методами.
- VO `CompanyProfile`: Title, Description, Emails[], Phones[], Socials[].
- Доменные ошибки через `var ErrXxx = errors.New(...)` (errors.Is-совместимо).

### Порты-потребителя (DIP, объявлены в usecase-пакете, не в domain)
- `Repository`: `UpsertPending(ctx, userID, domain)`, `ClaimDue(ctx, limit) ([]CompanyEnrichment)`, `Save(ctx, e)`, `Get(ctx, userID, domain) (CompanyEnrichment, found, err)`.
- `PageFetcher`: `Fetch(ctx, domain string) (page string, err error)` — адаптер над безопасным fetch из `internal/parser` (timeouts, body-limit, UA, proxy-aware httpClient).
- `Extractor`: `Extract(ctx, page string) (CompanyProfile, error)` — фаза 1 `HTMLExtractor`; фаза 2 `LLMExtractor` за тем же портом.

### usecase/
- `Enqueue(ctx, userID, email)`: `NewDomain(email)` → free/невалид = тихий скип; иначе `UpsertPending` если нет свежего (best-effort, ошибка логируется, НЕ ломает создание лида).
- `ProcessPending(ctx)`: `ClaimDue` батч → per-domain `ratelimit.Allow` → `Fetch`→`Extract` → `MarkEnriched`/`MarkFailed` (attempts++) → `Save`.
- `Get(ctx, userID, domain)`: чтение для карточки, tenant-scoped.

### Адаптеры / composition root
- `repository.go` (pgx) — `Repository`. Claim через `FOR UPDATE SKIP LOCKED` (multi-instance-safe).
- `website_fetcher.go` — `PageFetcher` поверх parser.
- `html_extractor.go` — `Extractor` (regex/meta).
- `cron.go` — `EnrichmentCron{Start(ctx)}` — зеркало `audit.RetentionCron`/`analytics.RefreshCron` (ticker, run-once при старте, graceful shutdown по ctx.Done()).
- Кросс-модуль: prospects/inbox объявляют мини-порт `EnrichmentEnqueuer interface { Enqueue(ctx, userID, email) }`, main.go инжектит enrichment UC (как existing identityLinker/SendGuard). НИКАКИХ прямых импортов enrichment из prospects/inbox.

## Данные — миграция 048

```sql
CREATE TABLE company_enrichment (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  domain varchar(255) NOT NULL,
  status varchar(20) NOT NULL DEFAULT 'pending',
  profile jsonb NOT NULL DEFAULT '{}',
  error text NOT NULL DEFAULT '',
  attempts int NOT NULL DEFAULT 0,
  enriched_at timestamptz,
  expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(user_id, domain)
);
CREATE INDEX idx_company_enrichment_pending ON company_enrichment (updated_at)
  WHERE status = 'pending';
```

ClaimDue: `status='pending'` OR (`status='enriched'` AND `expires_at < now()`) при `attempts < maxAttempts`. TTL refresh, retry-cap.

## Поток
1. **Enqueue** (синхронно, cheap, best-effort) — post-CreateProspect (prospects/usecase.go:~219) и post-CreateLead email-инлет (inbox/email.go:~426). Telegram-лид без email → нет домена → скип.
2. **Worker** — `EnrichmentCron` тик 30–60с, claim→ratelimit→fetch→extract→save.
3. **Read** — `GET /api/enrichment?domain=<>` (tenant-scoped по JWT userID), под AuthMiddleware.

## UI
Блок «О компании» на карточке лида/проспекта: описание + доп.контакты + соцсети + бейдж статуса (обогащается…/нет данных/ошибка). FE дёргает `/api/enrichment?domain=` (домен из email или из нового поля `companyDomain` в lead/prospect response — чистая функция, без зависимости от enrichment-модуля).

## TDD-план (RED→GREEN пары, гейт «2 коммита на поведение»)
1. `domain`: `NewDomain` (table-driven: corp ok / free-провайдеры отсев / мусор→err), `Status`-переходы, `MarkEnriched`/`MarkFailed`.
2. `HTMLExtractor` (unit, HTML-фикстуры → CompanyProfile; пустая/битая страница).
3. `Repository` (integration, throwaway DB): Upsert pending идемпотентно, ClaimDue (pending + stale), Save, Get tenant-scoped (чужой user не видит).
4. `ProcessPending` UC (fake Fetcher/Extractor + ratelimit): pending→enriched, fail→attempts++, skip свежих, claim due.
5. `Enqueue` UC: free-email скип, дедуп, best-effort (ошибка не ломает).
6. Handler read (tenant-scope) + wiring main (cron + порты-адаптеры).

## Verification
- `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`.
- unit: `go test ./internal/enrichment/...`.
- integration: throwaway pg (5433, floq:floq), `TEST_DATABASE_URL=... go test -tags=integration ./internal/enrichment/...`.
- FE: `node_modules/.bin/vitest run` + `npm run test:int`.
- `superpowers:code-reviewer` — все оси (TDD/DDD/CA) ≥8.

## Безопасность (SSRF) — два слоя защиты
Воркер фетчит `https://<домен из email>` — недоверенный egress (reachable даже неавторизованно через входящую почту). Защита:
1. **Domain VO** (`NewDomain`) отвергает IP-литералы и `host:port` (`net.ParseIP`/`ContainsAny(":[]")`) — компании-домен никогда не IP.
2. **Egress-guarded HTTP-клиент** (`newGuardedClient`): `net.Dialer.Control` режет loopback/private/link-local/unspecified по **resolved** IP (бьёт DNS-rebinding); редиректы capped (3) и каждый хоп ре-диалится через тот же guard (бьёт redirect-to-internal). Отдельный клиент, НЕ shared proxy-client (через прокси guard не видит resolved IP).

## Известные ограничения фазы 1 (hardening — фаза 2)
- **ClaimDue — single-worker** (cron сериализует тики): без `FOR UPDATE SKIP LOCKED`/lease. Multi-instance дал бы двойной скрейп — добавить processing-lease при горизонтальном масштабировании.
- **Partial index** покрывает только `status='pending'`; ветки `failed`/expired-enriched идут seq-scan'ом (ок на крошечной per-user таблице, индекс — при росте).

## Out of scope (фаза 2+)
- Rusprofile/ЕГРЮЛ, HH.ru источники (порт `Enricher`/`Extractor` готов их принять).
- LLM-экстракция (порт `Extractor` готов; audit-cost обвязка тогда).
- Платные API (Clearbit/Apollo) за флагом.
- Денормализация enriched-полей на сущности (пока read через `/api/enrichment`).
