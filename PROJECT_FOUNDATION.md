# Floq — PROJECT FOUNDATION v2.0

> AI-powered full-cycle sales assistant for small B2B businesses.
> Inbound: qualifies leads, drafts replies, tracks followups.
> Outbound: finds prospects, generates cold messages, runs sequences.

---

## 1. Product Vision

**One-liner:** Floq = inbound inbox + outbound outreach + AI brain — в одном инструменте.

**Problem:** Small B2B agencies lose revenue because:
- Leads arrive from multiple channels (Telegram, Email) with no single view
- Managers forget to follow up on inbound leads
- No system to qualify leads or draft replies quickly
- **Cold outreach is manual, slow, and not personalized**
- **No tool connects outbound prospecting with inbound pipeline**

**Solution:**
1. **Inbound:** Collects leads from Telegram + Email into one inbox, AI qualifies + drafts replies
2. **Outbound:** AI generates personalized cold messages, human confirms, sends via email
3. **Sequences:** Automated follow-up chains (Day 1 → Day 3 → Day 8) with human approval
4. **Unified pipeline:** Outbound prospect who replies → becomes inbound lead in same inbox
5. Reminds the manager if any lead goes silent for 2+ days

---

## 2. MVP Scope (Week 1–6)

### In scope — Inbound (done)
- [x] Unified inbox (Telegram + Email)
- [x] AI lead card (auto-qualification on first message)
- [x] AI reply draft with edit + send
- [x] Followup reminder (cron: 2 days silence → Telegram notification)
- [x] Pipeline stages: New → Qualified → In Conversation → Follow-up → Closed
- [x] Basic auth (email/password + JWT)
- [x] Single workspace per account

### In scope — Outbound
- [x] Prospects database (manual add + CSV import)
- [x] AI cold message generation (personalized by prospect data)
- [x] Sequences: multi-step outreach chains with configurable delays
- [x] Human approval before each send (with autopilot toggle)
- [ ] Email sending via Resend API
- [ ] Prospect → Lead conversion on reply (auto-enters inbound pipeline)
- [ ] Outbound dashboard: sequence stats, open/reply rates

### In scope — База и верификация (приоритет #1 по фидбеку PM)
- [ ] Свой email-верификатор (без платных API):
  - Синтаксис (RFC)
  - MX-запись домена (DNS lookup)
  - SMTP probe (RCPT TO без отправки)
  - Одноразовые домены (disposable filter)
  - Catch-all детекция
- [ ] Проверка Telegram username (resolveUsername)
- [ ] Скоринг проспекта: valid / risky / invalid
- [ ] Запрет добавления в секвенцию без прохождения верификации
- [ ] Дедупликация: внутри базы + с существующими лидами в inbox
- [ ] Bounce tracking: пометка невалидных после реальной отправки

### Парсинг контактов (MVP — по фидбеку PM)
- [ ] 2ГИС: парсинг по нише + городу (компания, телефон, адрес, категория)
- [ ] Сайты компаний: автопоиск email на странице контактов
- [ ] CSV: универсальный импорт из любых источников
- [ ] Контекст при парсинге: ниша, размер, описание деятельности → для ИИ-персонализации

### Мультиканальные секвенции (по фидбеку PM)
Секвенция — не просто цепочка email, а кросс-канальная воронка:
```
Шаг 1: Email (холодное письмо из контекста)
    ↓ нет ответа N дней
Шаг 2: Telegram (фоллоуап, менеджер отправляет сам, Floq готовит текст)
    ↓ нет ответа N дней
Шаг 3: Прозвон (карточка телемаркетологу: имя, телефон, контекст + что уже писали)
```

Каждый шаг имеет `channel`: "email" | "telegram" | "phone_call"
- email: отправляется автоматически (Resend)
- telegram: ИИ генерирует → менеджер копирует → отправляет сам
- phone_call: создаётся карточка задачи для телемаркетолога

### Контекстная персонализация (по фидбеку PM)
ИИ генерирует сообщения на основе контекста проспекта:
- Ниша / индустрия компании
- Размер компании (если известен)
- Должность контакта
- Что нашли при парсинге (описание с сайта, категория 2ГИС)
- История: что уже писали, через какой канал

### Out of scope (v1)
- LinkedIn automation, analytics dashboards, mobile app, CRM integrations, Telegram массовые DM (риск бана)

---

## 3. Stack (актуальные версии, проверено через Context7)

| Layer | Technology | Install |
|-------|-----------|---------|
| Backend | Go 1.26, modular monolith, Clean Architecture | — |
| HTTP Router | chi v5 | `go get -u github.com/go-chi/chi/v5` |
| AI | Pluggable provider interface (§4) | — |
| ↳ Claude | anthropic-sdk-go | `go get github.com/anthropics/anthropic-sdk-go@v1.4.0` |
| ↳ OpenAI / Ollama | openai-go | `go get github.com/openai/openai-go` |
| Telegram | go-telegram-bot-api v5 | `go get github.com/go-telegram-bot-api/telegram-bot-api/v5` |
| DB Driver | pgx/v5 | `go get github.com/jackc/pgx/v5` |
| Migrations | golang-migrate v4 | `go get github.com/golang-migrate/migrate/v4` |
| Email send | resend-go | `go get github.com/resendlabs/resend-go` |
| Config | godotenv | `go get github.com/joho/godotenv` |
| Frontend | Next.js 16 (App Router) | `npm install next@latest react@latest react-dom@latest` |
| Styling | Tailwind CSS v4.1 | `npm install tailwindcss@latest @tailwindcss/postcss@latest` |
| UI | shadcn/ui | `npx shadcn@latest init` |
| Database | PostgreSQL 18 | `docker: postgres:18-alpine` |
| Cache | Redis 8 | `docker: redis:8-alpine` |

---

## 4. AI Provider Abstraction

AI подключается через интерфейс. Провайдер переключается через `AI_PROVIDER` env без изменений кода.
Поддержка: Claude, OpenAI, любой OpenAI-compatible API (Ollama, Mistral, Llama и т.д.).

### Interface

```go
// /internal/ai/provider.go

type Message struct {
    Role    string // "system" | "user" | "assistant"
    Content string
}

type CompletionRequest struct {
    Messages  []Message
    MaxTokens int
}

type Provider interface {
    Complete(ctx context.Context, req CompletionRequest) (string, error)
    Name() string
}
```

### Providers

```
/internal/ai/providers/
  claude.go   — anthropic-sdk-go (ModelClaude3_7SonnetLatest)
  openai.go   — openai-go (gpt-4o или любая модель)
  ollama.go   — openai-go с baseURL=http://localhost:11434/v1 (llama3.2, mistral, etc.)
```

### Claude provider (пример)

```go
import (
    "github.com/anthropics/anthropic-sdk-go"
    "github.com/anthropics/anthropic-sdk-go/option"
)

func NewClaudeProvider(apiKey string) *ClaudeProvider {
    return &ClaudeProvider{
        client: anthropic.NewClient(option.WithAPIKey(apiKey)),
        model:  anthropic.ModelClaude3_7SonnetLatest,
    }
}
```

### Ollama provider (бесплатно, опенсорс)

```go
import "github.com/openai/openai-go"
import "github.com/openai/openai-go/option"

func NewOllamaProvider(model string) *OllamaProvider {
    client := openai.NewClient(
        option.WithBaseURL("http://localhost:11434/v1"),
        option.WithAPIKey("ollama"),
    )
    return &OllamaProvider{client: client, model: model}
}
```

### main.go инициализация

```go
var aiProvider ai.Provider
switch cfg.AIProvider {
case "claude":
    aiProvider = providers.NewClaudeProvider(cfg.AnthropicAPIKey)
case "openai":
    aiProvider = providers.NewOpenAIProvider(cfg.OpenAIAPIKey, cfg.OpenAIModel)
case "ollama":
    aiProvider = providers.NewOllamaProvider(cfg.OllamaModel)
default:
    log.Fatal("unknown AI_PROVIDER:", cfg.AIProvider)
}
```

---

## 5. Monolith Modules

```
/internal
  /leads        — lead entity, pipeline stages
  /inbox        — ingestion: Telegram, Email IMAP
  /ai
    provider.go     — interface
    prompts.go      — все промпты как константы (inbound + outbound)
    client.go       — qualify / draft / followup / cold message usecases
    /providers
      claude.go
      openai.go
      ollama.go
  /prospects    — prospect entity, CSV import, manual CRUD
  /verify       — email/TG верификация (DNS, SMTP probe, disposable filter)
  /sequences    — outreach sequences, steps, scheduling
  /outbound     — cold message sending, approval queue, bounce tracking
  /reminders    — cron followup alerts
  /auth         — JWT, users
  /notify       — Telegram notifications to manager
```

---

## 6. Data Models

```go
type Lead struct {
    ID           uuid.UUID
    Channel      string     // "telegram" | "email"
    ContactName  string
    Company      string
    FirstMessage string
    Status       LeadStatus // new|qualified|in_conversation|followup|closed
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

type AIQualification struct {
    LeadID            uuid.UUID
    IdentifiedNeed    string
    EstimatedBudget   string
    Deadline          string
    Score             int
    ScoreReason       string
    RecommendedAction string
    ProviderUsed      string
    GeneratedAt       time.Time
}

type Message struct {
    ID        uuid.UUID
    LeadID    uuid.UUID
    Direction string // "inbound" | "outbound"
    Body      string
    SentAt    time.Time
}

type AIDraft struct {
    LeadID    uuid.UUID
    Body      string
    CreatedAt time.Time
}

// --- Outbound models ---

type Prospect struct {
    ID          uuid.UUID
    UserID      uuid.UUID
    Name        string
    Company     string
    Title       string     // должность
    Email       string
    Phone       string     // для передачи на прозвон
    TelegramUsername string
    Industry    string     // ниша (из парсинга)
    CompanySize string     // размер компании
    Context     string     // доп. контекст для ИИ (описание с сайта, категория 2ГИС)
    Source      string     // "manual" | "csv"
    Source      string     // "manual" | "csv"
    Status      string     // "new" | "in_sequence" | "replied" | "converted" | "opted_out"
    VerifyStatus string    // "not_checked" | "valid" | "risky" | "invalid"
    VerifyDetails string   // JSON: {"mx":true,"smtp":true,"disposable":false,"catch_all":false}
    ConvertedLeadID *uuid.UUID // → leads.id после ответа
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// --- Verification ---

type VerifyResult struct {
    Email          string
    IsValidSyntax  bool   // RFC формат
    HasMX          bool   // MX-запись существует
    SMTPValid      bool   // RCPT TO вернул 250
    IsDisposable   bool   // одноразовый домен
    IsCatchAll     bool   // сервер принимает всё
    IsFreeProvider bool   // gmail, mail.ru, yandex
    Score          int    // 0-100, итоговый скор
    Status         string // "valid" | "risky" | "invalid"
}

type Sequence struct {
    ID          uuid.UUID
    UserID      uuid.UUID
    Name        string
    IsActive    bool
    CreatedAt   time.Time
}

type SequenceStep struct {
    ID          uuid.UUID
    SequenceID  uuid.UUID
    StepOrder   int        // 1, 2, 3...
    Channel     string     // "email" | "telegram" | "phone_call"
    DelayDays   int        // дней после предыдущего шага
    PromptHint  string     // подсказка для ИИ ("первое касание", "мягкий фоллоуап", "финальное")
    CreatedAt   time.Time
}

type OutboundMessage struct {
    ID          uuid.UUID
    ProspectID  uuid.UUID
    SequenceID  uuid.UUID
    StepOrder   int
    Body        string     // сгенерированный ИИ текст
    Status      string     // "draft" | "approved" | "sent" | "rejected"
    ScheduledAt time.Time  // когда отправить
    SentAt      *time.Time
    CreatedAt   time.Time
}
```

---

## 7. AI Prompts (/internal/ai/prompts.go)

```go
const QualificationSystem = `You are a sales qualification assistant for a B2B web development agency.
Analyze the incoming lead message and extract key information.
Respond ONLY with valid JSON, no markdown, no preamble.`

const QualificationUser = `Incoming message from {{contact_name}} via {{channel}}:
"{{first_message}}"
Return: {"identified_need":"...","estimated_budget":"...","deadline":"...","score":0,"score_reason":"...","recommended_action":"..."}`

const DraftSystem = `You are a sales assistant for a B2B web development agency.
Write a warm reply in Russian. 3-5 sentences. Ask one clarifying question.
No prices. No bureaucratic language. Only the message text, no preamble.`

const DraftUser = `Lead: {{contact_name}}, {{company}} | Channel: {{channel}}
Message: "{{first_message}}"
Qualification: {{qualification_json}}
Write a reply acknowledging their need with one smart clarifying question.`

const FollowupSystem = `You are a sales assistant. Write a short followup message in Russian. Only the text, no preamble.`

const FollowupUser = `Lead: {{contact_name}}, {{company}}
Their last message ({{days_ago}} days ago): "{{last_message}}"
Our last reply: "{{our_last_reply}}"
Write a brief non-pushy followup to re-engage.`

// --- Outbound prompts ---

const ColdOutreachSystem = `You are a B2B sales development rep writing cold outreach emails in Russian.
Write a personalized, concise email (3-5 sentences). Be warm but professional.
Reference something specific about the prospect's company or role.
End with a clear, low-commitment CTA (e.g. short call, quick question).
No prices. No hard sell. Only the email text, no subject line, no preamble.`

const ColdOutreachUser = `Prospect: {{name}}, {{title}} at {{company}}
Step: {{step_hint}}
{{#if previous_message}}Previous message we sent: "{{previous_message}}"{{/if}}
Write a personalized outreach message.`
```

---

## 8. API Routes

```
POST   /api/auth/login
POST   /api/auth/refresh

GET    /api/leads
GET    /api/leads/:id
PATCH  /api/leads/:id/status

GET    /api/leads/:id/messages
POST   /api/leads/:id/send

GET    /api/leads/:id/qualification
POST   /api/leads/:id/qualify

GET    /api/leads/:id/draft
POST   /api/leads/:id/draft/regen

GET    /api/reminders
POST   /api/reminders/:id/snooze
POST   /api/reminders/:id/dismiss

GET    /api/settings/channels
POST   /api/settings/telegram
POST   /api/settings/email

# --- Outbound ---

GET    /api/prospects
POST   /api/prospects                  # add single prospect
POST   /api/prospects/import           # CSV import
GET    /api/prospects/:id
DELETE /api/prospects/:id

GET    /api/sequences
POST   /api/sequences                  # create sequence
GET    /api/sequences/:id
PUT    /api/sequences/:id
DELETE /api/sequences/:id
POST   /api/sequences/:id/steps        # add step
POST   /api/sequences/:id/launch       # launch for selected prospects
PATCH  /api/sequences/:id/toggle       # activate/pause

GET    /api/outbound/queue             # messages awaiting approval
POST   /api/outbound/:id/approve       # approve & schedule send
POST   /api/outbound/:id/reject        # reject message
POST   /api/outbound/:id/edit          # edit before approve
GET    /api/outbound/stats             # sent/opened/replied counts
```

---

## 9. Frontend Pages

```
/app
  (auth)/login/page.tsx
  (dashboard)/
    inbox/page.tsx
    inbox/[leadId]/page.tsx
    alerts/page.tsx
    prospects/page.tsx              # NEW: prospect list + import
    prospects/[prospectId]/page.tsx # NEW: prospect detail
    sequences/page.tsx              # NEW: sequence list
    sequences/[seqId]/page.tsx      # NEW: sequence builder
    outbound/page.tsx               # NEW: approval queue
  inbox/page.tsx
  inbox/[leadId]/page.tsx
  alerts/page.tsx
  settings/page.tsx

/components
  /layout       — Sidebar.tsx, TopBar.tsx
  /leads        — LeadCard, LeadDetail, QualificationBlock,
                  ConversationThread, AIDraftPanel
  /alerts       — AlertCard.tsx
  /ui           — shadcn/ui
```

---

## 10. Project Structure

```
floq/
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── leads/handler.go, usecase.go, repository.go
│   │   ├── inbox/telegram.go, email.go
│   │   ├── ai/
│   │   │   ├── provider.go
│   │   │   ├── prompts.go
│   │   │   ├── client.go
│   │   │   └── providers/claude.go, openai.go, ollama.go
│   │   ├── reminders/cron.go
│   │   ├── auth/handler.go, middleware.go
│   │   └── notify/telegram.go
│   ├── migrations/
│   │   ├── 001_create_users.up.sql
│   │   ├── 002_create_leads.up.sql
│   │   ├── 003_create_messages.up.sql
│   │   └── 004_create_qualifications.up.sql
│   └── go.mod
├── frontend/
│   ├── app/
│   ├── components/
│   ├── lib/api.ts
│   └── package.json
├── docker-compose.yml
└── .env.example
```

---

## 11. docker-compose.yml

```yaml
services:
  postgres:
    image: postgres:18-alpine
    environment:
      POSTGRES_DB: floq
      POSTGRES_USER: floq
      POSTGRES_PASSWORD: floq
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:8-alpine
    ports:
      - "6379:6379"

volumes:
  postgres_data:
```

---

## 12. .env.example

```env
APP_PORT=8080
APP_ENV=development

DATABASE_URL=postgres://floq:floq@localhost:5432/floq?sslmode=disable
REDIS_URL=redis://localhost:6379

JWT_SECRET=change-me
JWT_EXPIRES_IN=15m
JWT_REFRESH_EXPIRES_IN=7d

# AI Provider: claude | openai | ollama
AI_PROVIDER=claude

ANTHROPIC_API_KEY=sk-ant-...

OPENAI_API_KEY=sk-...
OPENAI_MODEL=gpt-4o

OLLAMA_BASE_URL=http://localhost:11434
OLLAMA_MODEL=llama3.2

TELEGRAM_BOT_TOKEN=...

RESEND_API_KEY=re_...
SMTP_FROM=leads@yourdomain.com

IMAP_HOST=imap.gmail.com
IMAP_PORT=993
IMAP_USER=...
IMAP_PASSWORD=...
```

---

## 13. Монетизация

| Plan | Price | Limit |
|------|-------|-------|
| Starter | 3 900 ₽/mo | 200 leads |
| Growth | 7 900 ₽/mo | 1 000 leads |
| Pro | 14 900 ₽/mo | Unlimited + API |

---

## 14. Claude Code Starter Prompt

```
You are helping build Floq — an AI-powered inbound sales assistant for small B2B businesses.
Reference PROJECT_FOUNDATION.md for all decisions. Do not deviate from it.

Tech stack (verified):
- Go 1.26, modular monolith, Clean Architecture
- Router: github.com/go-chi/chi/v5
- AI: pluggable Provider interface — claude / openai / ollama (see §4)
  - Claude: github.com/anthropics/anthropic-sdk-go@v1.4.0
  - OpenAI + Ollama: github.com/openai/openai-go
- DB: PostgreSQL 18, pgx/v5, golang-migrate/v4 (no ORM, raw SQL)
- Cache: Redis 8
- Telegram: github.com/go-telegram-bot-api/telegram-bot-api/v5
- Email: resendlabs/resend-go + IMAP polling
- Frontend: Next.js 16 (App Router), TypeScript
- Tailwind CSS v4.1 (@tailwindcss/postcss)
- shadcn/ui (npx shadcn@latest init)

Architecture rules:
- Clean Architecture: handler → usecase → repository
- AI is always called through Provider interface, never SDK directly in usecases
- All prompts in /internal/ai/prompts.go as string constants (system + user separate)
- AI qualification is async on ingestion, result stored to DB
- No ORM — raw SQL + golang-migrate
- AI_PROVIDER env switches provider at startup

Build in this order:
1. Go 1.26 module init + full project structure
2. docker-compose.yml (postgres:18-alpine + redis:8-alpine)
3. DB migrations: users, leads, messages, qualifications, drafts, reminders
4. /internal/ai — Provider interface + claude/openai/ollama implementations
5. /internal/leads — repository + usecase + chi handler
6. /internal/inbox — Telegram Bot long-polling
7. /internal/auth — JWT middleware + login
8. /internal/reminders — cron (hourly, silence > 2 days)
9. /internal/notify — Telegram notifications to manager
10. Frontend: Next.js 16 App Router — inbox, lead detail, alerts pages
```
