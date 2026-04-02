# Floq — Roadmap

> Порядок реализации по приоритетам PM.
> Обновлено: 2026-04-02

---

## Статус: что готово

### Backend (Go)
| Модуль | Файлы | Статус |
|---|---|---|
| Конфиг + main.go | `cmd/server/main.go`, `internal/config/` | Скелет, модули не подключены |
| Auth (JWT) | `internal/auth/` | Код готов |
| Leads CRUD | `internal/leads/` | Код готов |
| Inbox (TG + Email) | `internal/inbox/` | Код готов (email IMAP — стаб) |
| AI Provider | `internal/ai/` | Код готов (Claude, OpenAI, Ollama) |
| Reminders cron | `internal/reminders/` | Код готов |
| Notify (TG) | `internal/notify/` | Код готов |
| Prospects CRUD | `internal/prospects/` | Код готов |
| Sequences | `internal/sequences/` | Код готов |
| Миграции | `migrations/001-009` | Готовы |

### Frontend (Next.js)
| Страница | URL | Статус |
|---|---|---|
| Login | `/login` | Готова (мок) |
| Inbox | `/inbox` | Готова (мок) |
| Lead Detail | `/inbox/[leadId]` | Готова (мок) |
| Alerts | `/alerts` | Готова (мок) |
| Проспекты | `/prospects` | Готова (мок) |
| Секвенции | `/sequences` | Готова (мок) |
| Очередь отправки | `/outbound` | Готова (мок) |

### Infra
| Компонент | Статус |
|---|---|
| docker-compose (PG + Redis) | Готов |
| .env.example | Готов |

---

## Сессия 1: Верификатор email + TG (ПРИОРИТЕТ PM)

> "50-60% хорошего outreach — это сбор релевантной базы"
> "Дв шлёт 300 писем/день по мусорной базе → выхлоп 0"
> "Сервисы для почт платные — нам бы свой"

### Backend
- [ ] `internal/verify/email.go` — верификация email:
  - Синтаксис (RFC regex)
  - MX lookup (`net.LookupMX`)
  - SMTP probe (RCPT TO без отправки)
  - Одноразовые домены (список ~3000)
  - Catch-all детекция (RCPT TO на рандомный адрес)
  - Free provider детекция (gmail, mail.ru, yandex)
  - Скоринг 0-100 → статус valid/risky/invalid
- [ ] `internal/verify/telegram.go` — проверка TG username
- [ ] `internal/verify/disposable.go` — список одноразовых доменов
- [ ] `internal/verify/handler.go` — API endpoints:
  - POST /api/verify/email (один адрес)
  - POST /api/verify/batch (массовая проверка проспектов)
  - GET /api/prospects/:id/verify (статус проверки)
- [ ] Миграция 010: добавить verify_status, verify_details, telegram_username в prospects

### Frontend
- [ ] Обновить `/prospects` — колонка "Проверка" с иконками (галочка/вопрос/крестик)
- [ ] Кнопка "Проверить базу" — запуск массовой верификации
- [ ] Проспект нельзя добавить в секвенцию если не проверен

---

## Сессия 2: Парсинг контактов (ПРИОРИТЕТ PM)

> "парсинг контактов надо сразу внедрять"
> "надо генерить идеи откуда брать контакты"

### Backend
- [ ] `internal/parser/twogis.go` — парсинг 2ГИС:
  - Вход: ниша + город
  - Выход: компания, телефон, адрес, категория, сайт
- [ ] `internal/parser/website.go` — поиск email на сайтах:
  - Вход: URL сайта
  - Выход: найденные email-адреса (со страницы контактов)
- [ ] `internal/parser/handler.go` — API:
  - POST /api/parser/twogis {query, city} → список компаний
  - POST /api/parser/website {url} → найденные email
  - POST /api/parser/import → сохранить результаты как проспектов
- [ ] Контекст: при парсинге сохраняем industry, company_size, context в проспекта

### Frontend
- [ ] Новая страница `/parser` или модалка в `/prospects`:
  - Форма: ниша + город → "Найти компании"
  - Результаты таблицей → "Добавить в проспекты"
  - Поиск email по URL

---

## Сессия 3: Мультиканальные секвенции

> "слать письмо, потом слать в тг фоллоу ап или давать на прозвон"
> "письмо должно быть исходя из контекста"

### Backend
- [ ] Обновить sequence_steps: добавить поле `channel` (email/telegram/phone_call)
- [ ] Миграция: ALTER TABLE sequence_steps ADD channel
- [ ] Обновить launch: для каждого шага разное поведение:
  - email → создать outbound_message (автоотправка)
  - telegram → создать outbound_message с пометкой "скопировать"
  - phone_call → создать задачу для телемаркетолога (имя, телефон, контекст)
- [ ] Обновить AI промпты: передавать context проспекта для персонализации
- [ ] `internal/tasks/` — модуль задач на прозвон для телемаркетологов

### Frontend
- [ ] Обновить `/sequences` — выбор канала для каждого шага (email/TG/звонок)
- [ ] Обновить `/outbound` — разные иконки и действия по каналу:
  - Email: "Подтвердить" (автоотправка)
  - Telegram: "Скопировать" (в буфер)
  - Звонок: "Карточка прозвона" (имя, телефон, что писали)
- [ ] Новая страница `/calls` — очередь прозвонов для телемаркетологов

---

## Сессия 4: Связать backend + фронт

### main.go
- [ ] Подключить к роутеру: все модули (prospects, sequences, outbound, verify, parser)
- [ ] Инициализация DB pool, AI provider, crons
- [ ] Outbound sender cron (email через Resend)
- [ ] Bounce tracking

### Frontend → Backend
- [ ] Убрать моки, подключить реальный API
- [ ] Реальный логин + seed data
- [ ] E2E: парсинг → верификация → секвенция → отправка

---

## Сессия 5: Полировка + демо

- [ ] Дедупликация: не писать тому, кто уже в inbox
- [ ] Error handling + loading states
- [ ] Seed data для демо PM
- [ ] Проспект ответил боту → auto-convert в лид

---

## Будущее (v2)

- [ ] HH.ru парсинг (компании с вакансиями = есть бюджет)
- [ ] Rusprofile (ИНН, выручка, размер)
- [ ] LinkedIn Sales Navigator экспорт
- [ ] Аналитика: open rate, reply rate, conversion rate
- [ ] A/B тестирование промптов в секвенциях
- [ ] Multi-workspace (команды)
- [ ] Webhook для интеграций
- [ ] Автоматический enrichment: по домену компании дополнять данные
