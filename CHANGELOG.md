# Changelog

Хронологический индекс релизов floq — «что и когда». Каждая фича выходит отдельным релизом (тематическую карту см. в [ROADMAP.md](ROADMAP.md)).

Полные заметки с обоснованием «почему и как» — в [GitHub Releases](https://github.com/VDV001/floq/releases) и в git-истории (`git log v<old>..v<new>`). Версия в этом файле совпадает с [VERSION](VERSION).

Формат основан на [Keep a Changelog](https://keepachangelog.com/); версии — SemVer-подобные.

## [0.81.0] — 2026-07-02
Онбординг: шаг «Настройте отправку писем» помечается «Готово» только после успешной проверки Resend, а не при заполненном ключе — миграция 056 resend_verified; закрывает последний канал из #222 (#241)

## [0.80.0] — 2026-07-01
Онбординг: шаг канала (AI/SMTP/IMAP) помечается «Готово» только после успешной проверки подключения, а не при заполненных полях — миграция 055 добавляет persist-флаги verified; Ollama больше не «готово» без пинга, невалидный ключ не считается настроенным (#222)

## [0.79.0] — 2026-07-01
Облачные AI-провайдеры (OpenAI/Claude/Groq): тест подключения через бесплатный health-check (/models) вместо платной генерации; понятные сообщения об ошибке ключа, лимите запросов и недоступности сервиса вместо сырого текста SDK (#235)

## [0.78.0] — 2026-07-01
Ollama: тест подключения через быстрый health-check (/api/tags) вместо полной генерации — больше нет ложного «context deadline exceeded» на холодном старте модели; понятные сообщения о недоступности сервера и не скачанной модели (#227)

## [0.77.0] — 2026-07-01
Honest launch outcome — Launch reports queued/skipped so the UI stops showing a false success when nothing is queued; ineligible prospects are flagged with the reason (#221)

## [0.76.0] — 2026-06-28
Domain-owned retention GC terminal set (#212)

## [0.75.0] — 2026-06-28
Multi-worker leased claim (#212 part 2)

## [0.74.0] — 2026-06-28
Terminal-job retention GC (#212 part 1)

## [0.73.0] — 2026-06-28
Bounded intake retry cap / quarantine (#208)

## [0.72.0] — 2026-06-28
durable auto-qualification worker (closes #206)

## [0.71.0] — 2026-06-27
transactional telegram lead.created intake

## [0.70.0] — 2026-06-27
transactional email lead.created intake

## [0.69.0] — 2026-06-27
Transactional outbox (#199)

## [0.68.0] — 2026-06-27
Webhooks sequence.completed (#197)

## [0.67.0] — 2026-06-27
Webhooks endpoint active-toggle (#201)

## [0.66.0] — 2026-06-27
Webhooks delivery claim due-index (#198)

## [0.65.0] — 2026-06-27
Webhooks Phase 3 (Settings UI)

## [0.64.0] — 2026-06-27
Webhooks Phase 2 (domain event emission)

## [0.63.0] — 2026-06-27
Outgoing webhooks Phase 1 (delivery infrastructure)

## [0.62.0] — 2026-06-27
Онбординг: troubleshooting FAQ

## [0.61.0] — 2026-06-26
подсказка «Пройдите обучение» на Входящих

## [0.60.0] — 2026-06-26
развёрнутый туториал /onboarding

## [0.59.0] — 2026-06-26
Outbound HITL (per-sequence approval gate)

## [0.58.0] — 2026-06-26
DaData registry Prometheus counter

## [0.57.0] — 2026-06-26
Auto-enrichment phase 3: legal requisites (DaData)

## [0.56.0] — 2026-06-26
Auto-enrichment phase 2: LLMExtractor

## [0.55.0] — 2026-06-26
Auto-enrichment Phase 1 (#182)

## [0.54.0] — 2026-06-26
KEK rotation (#178)

## [0.53.0] — 2026-06-26
Secrets-at-rest: drop plaintext columns

## [0.52.0] — 2026-06-26
Экран архивных лидов + разархивирование

## [0.51.0] — 2026-06-26
настоящий архив лида

## [0.50.0] — 2026-06-25
фильтр напоминаний по срочности

## [0.49.2] — 2026-06-25
Broader sequences IDOR fix

## [0.49.1] — 2026-06-25
IDOR fix (launch)

## [0.49.0] — 2026-06-25
Period-окна воронки

## [0.48.0] — 2026-06-25
Ясность навигации

## [0.47.0] — 2026-06-25
Автопилот

## [0.46.0] — 2026-06-18
L2 tool-call firewall reply-path

## [0.45.0] — 2026-06-18
agent-security guardrails pilot

## [0.44.0] — 2026-06-07
(служебный релиз — см. git-историю)

## [0.43.0] — 2026-06-06
(служебный релиз — см. git-историю)

## [0.42.0] — 2026-06-06
шифрование секретов клиента at-rest

## [0.41.0] — 2026-06-04
(служебный релиз — см. git-историю)

## [0.40.0] — 2026-06-04
(служебный релиз — см. git-историю)

## [0.39.0] — 2026-06-04
(служебный релиз — см. git-историю)

## [0.38.0] — 2026-06-04
(служебный релиз — см. git-историю)

## [0.37.0] — 2026-06-04
(служебный релиз — см. git-историю)

## [0.36.0] — 2026-06-04
(служебный релиз — см. git-историю)

## [0.35.0] — 2026-06-04
(служебный релиз — см. git-историю)

## [0.34.0] — 2026-06-04
(служебный релиз — см. git-историю)

## [0.33.1] — 2026-06-04
(служебный релиз — см. git-историю)

## [0.33.0] — 2026-06-04
(служебный релиз — см. git-историю)

## [0.32.0] — 2026-06-04
(служебный релиз — см. git-историю)

## [0.31.0] — 2026-06-04
(служебный релиз — см. git-историю)

## [0.30.0] — 2026-06-03
(служебный релиз — см. git-историю)

## [0.29.0] — 2026-06-03
(служебный релиз — см. git-историю)

## [0.28.0] — 2026-05-20
(служебный релиз — см. git-историю)

## [0.27.1] — 2026-05-20
(служебный релиз — см. git-историю)

## [0.27.0] — 2026-05-20
(служебный релиз — см. git-историю)

## [0.26.0] — 2026-05-20
(служебный релиз — см. git-историю)

## [0.25.1] — 2026-05-19
(служебный релиз — см. git-историю)

## [0.25.0] — 2026-05-19
(служебный релиз — см. git-историю)

## [0.24.5] — 2026-05-19
(служебный релиз — см. git-историю)

## [0.24.4] — 2026-05-19
(служебный релиз — см. git-историю)

## [0.24.3] — 2026-05-19
(служебный релиз — см. git-историю)

## [0.24.2] — 2026-05-19
(служебный релиз — см. git-историю)

## [0.24.1] — 2026-05-19
(служебный релиз — см. git-историю)

## [0.24.0] — 2026-05-19
GET /api/audit/cost-summary endpoint

## [0.23.1] — 2026-05-19
Nil-guard Approve/Reject decider

## [0.23.0] — 2026-05-19
Inbox-list badge + post-decision refetch (HITL UX)

## [0.22.0] — 2026-05-19
decided_by operator attribution on pending_replies

## [0.21.0] — 2026-05-19
Resend Idempotency-Key + bounded retry

## [0.20.0] — 2026-05-19
Rate-limit on HITL approve/reject

## [0.19.0] — 2026-05-17
HITL approval gate for booking-link auto-reply

## [0.18.1] — 2026-05-17
audit compliance fixups (TDD/DDD/CA/Enterprise >=9)

## [0.18.0] — 2026-05-17
AI cost-tracking audit log

## [0.17.0] — 2026-05-16
Phase 2 multi-source aggregation (closes #27) + IDOR closure

## [0.16.0] — 2026-05-16
IdentityResolver wiring + backfill (#38, #27 PR2)

## [0.15.0] — 2026-05-16
Identity foundation + inbox normalize (#27 PR1, P0-2)

## [0.14.1] — 2026-05-16
async qualify parent ctx (P0-5)

## [0.14.0] — 2026-05-16
attachments wiring (issue #25 closed)

## [0.13.0] — 2026-05-16
attachments analyzer (PR A: library)

## [0.12.0] — 2026-05-16
LLM style-check pass for outreach

## [0.11.0] — 2026-05-16
CSV fallback name + contact normalization

## [0.10.0] — 2026-05-15
security pilot (agent-security-defaults v1.0)

## [0.9.0] — 2026-05-15
pluggable ModelMode (Plan/Execute/Budget)

## [0.8.2] — 2026-05-02
tech debt cleanup (TDD/DDD/CA → 10/10)

## [0.8.1] — 2026-05-02
proxy fixes (SMTP context, port 587) + CLA infra

## [0.8.0] — 2026-05-02
Proxy support + AGPL-3.0 license (issue #17)

## [0.7.0] — 2026-05-02
Flexible CSV import (issue #16)

## [0.6.0] — 2026-04-17
(служебный релиз — см. git-историю)

