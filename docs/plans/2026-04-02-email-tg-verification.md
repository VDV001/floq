# Email + Telegram Verification Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build an email and Telegram username verification system that scores prospects as valid/risky/invalid, integrated into the prospects module and frontend.

**Architecture:** New `internal/verify/` module with Clean Architecture (handler -> usecase -> pure functions). Email verification pipeline: syntax check -> MX lookup -> SMTP probe -> disposable filter -> catch-all detection -> free provider detection -> scoring. TG verification via Telegram Bot API `getChat`. Migration adds verification fields to prospects table. Frontend gets verification column + batch verify button.

**Tech Stack:** Go 1.26, net/net.LookupMX, net/smtp for SMTP probe, go-telegram-bot-api for TG check, pgx/v5 for DB, chi/v5 for routing, Next.js 16 + Tailwind 4 for frontend.

---

## Task 1: Migration 010 — Add verification fields to prospects

**Files:**
- Create: `backend/migrations/010_add_verification_fields.up.sql`
- Create: `backend/migrations/010_add_verification_fields.down.sql`

**Step 1: Write the up migration**

```sql
-- 010_add_verification_fields.up.sql
CREATE TYPE verify_status AS ENUM ('not_checked', 'valid', 'risky', 'invalid');

ALTER TABLE prospects
    ADD COLUMN phone VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN telegram_username VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN industry VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN company_size VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN context TEXT NOT NULL DEFAULT '',
    ADD COLUMN verify_status verify_status NOT NULL DEFAULT 'not_checked',
    ADD COLUMN verify_score INT NOT NULL DEFAULT 0,
    ADD COLUMN verify_details JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN verified_at TIMESTAMPTZ;

CREATE INDEX idx_prospects_verify_status ON prospects(verify_status);
CREATE INDEX idx_prospects_telegram_username ON prospects(telegram_username) WHERE telegram_username != '';
```

**Step 2: Write the down migration**

```sql
-- 010_add_verification_fields.down.sql
ALTER TABLE prospects
    DROP COLUMN IF EXISTS phone,
    DROP COLUMN IF EXISTS telegram_username,
    DROP COLUMN IF EXISTS industry,
    DROP COLUMN IF EXISTS company_size,
    DROP COLUMN IF EXISTS context,
    DROP COLUMN IF EXISTS verify_status,
    DROP COLUMN IF EXISTS verify_score,
    DROP COLUMN IF EXISTS verify_details,
    DROP COLUMN IF EXISTS verified_at;

DROP TYPE IF EXISTS verify_status;
```

**Step 3: Commit**

```bash
git add backend/migrations/010_add_verification_fields.up.sql backend/migrations/010_add_verification_fields.down.sql
git commit -m "feat: add migration 010 for verification fields on prospects"
```

---

## Task 2: Update Prospect struct and repository to include new fields

**Files:**
- Modify: `backend/internal/prospects/repository.go`

The Prospect struct currently has 11 fields. We need to add: Phone, TelegramUsername, Industry, CompanySize, Context, VerifyStatus, VerifyScore, VerifyDetails, VerifiedAt.

**Step 1: Update the Prospect struct**

Add these fields to the existing struct in `repository.go`:

```go
type Prospect struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	Name             string     `json:"name"`
	Company          string     `json:"company"`
	Title            string     `json:"title"`
	Email            string     `json:"email"`
	Phone            string     `json:"phone"`
	TelegramUsername string     `json:"telegram_username"`
	Industry         string     `json:"industry"`
	CompanySize      string     `json:"company_size"`
	Context          string     `json:"context"`
	Source           string     `json:"source"`
	Status           string     `json:"status"`
	VerifyStatus     string     `json:"verify_status"`
	VerifyScore      int        `json:"verify_score"`
	VerifyDetails    string     `json:"verify_details"` // JSON string
	VerifiedAt       *time.Time `json:"verified_at,omitempty"`
	ConvertedLeadID  *uuid.UUID `json:"converted_lead_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
```

**Step 2: Update all SQL queries in repository**

All SELECT queries need the new columns. All INSERT queries need the new columns. Update `ListProspects`, `GetProspect`, `CreateProspect` to include all new columns in both query and Scan.

Column order in SELECT/INSERT:
```
id, user_id, name, company, title, email, phone, telegram_username, industry, company_size, context, source, status, verify_status, verify_score, verify_details, verified_at, converted_lead_id, created_at, updated_at
```

**Step 3: Add UpdateVerification method to repository**

```go
func (r *Repository) UpdateVerification(ctx context.Context, id uuid.UUID, verifyStatus string, verifyScore int, verifyDetails string, verifiedAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE prospects SET verify_status = $1, verify_score = $2, verify_details = $3, verified_at = $4, updated_at = $5 WHERE id = $6`,
		verifyStatus, verifyScore, verifyDetails, verifiedAt, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update verification: %w", err)
	}
	return nil
}
```

**Step 4: Update handler.go createProspect to accept new fields**

Update the create body struct to include phone, telegram_username, industry, company_size, context. Wire them into the Prospect construction.

**Step 5: Update usecase.go ImportCSV to handle new optional columns**

CSV header now supports optional columns: phone, telegram_username, industry, company_size, context. The first 4 (name, company, title, email) are still required. Extra columns are mapped by header name.

**Step 6: Commit**

```bash
git add backend/internal/prospects/
git commit -m "feat: add verification and enrichment fields to prospects module"
```

---

## Task 3: Disposable email domain list

**Files:**
- Create: `backend/internal/verify/disposable.go`

**Step 1: Create the disposable domain checker**

```go
package verify

// IsDisposable returns true if the domain is a known disposable email provider.
func IsDisposable(domain string) bool {
	_, ok := disposableDomains[domain]
	return ok
}

// disposableDomains is a set of known disposable/temporary email domains.
// Source: curated from public lists (github.com/disposable-email-domains/disposable-email-domains)
var disposableDomains = map[string]struct{}{
	// Include ~200 most common disposable domains
	"tempmail.com": {}, "guerrillamail.com": {}, "mailinator.com": {},
	"throwaway.email": {}, "yopmail.com": {}, "10minutemail.com": {},
	// ... (full list embedded in code)
}
```

Include the top ~200-300 most common disposable domains. The full list should be embedded as a Go map for zero-dependency operation.

**Step 2: Commit**

```bash
git add backend/internal/verify/disposable.go
git commit -m "feat: add disposable email domain list"
```

---

## Task 4: Email verification core logic

**Files:**
- Create: `backend/internal/verify/email.go`

**Step 1: Write the email verification module**

```go
package verify

import (
	"fmt"
	"net"
	"net/smtp"
	"regexp"
	"strings"
	"time"
)

// EmailResult holds the result of email verification checks.
type EmailResult struct {
	Email          string `json:"email"`
	IsValidSyntax  bool   `json:"is_valid_syntax"`
	HasMX          bool   `json:"has_mx"`
	SMTPValid      bool   `json:"smtp_valid"`
	SMTPError      string `json:"smtp_error,omitempty"`
	IsDisposable   bool   `json:"is_disposable"`
	IsCatchAll     bool   `json:"is_catch_all"`
	IsFreeProvider bool   `json:"is_free_provider"`
	Score          int    `json:"score"`
	Status         string `json:"status"` // "valid" | "risky" | "invalid"
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

var freeProviders = map[string]struct{}{
	"gmail.com": {}, "yahoo.com": {}, "hotmail.com": {}, "outlook.com": {},
	"mail.ru": {}, "yandex.ru": {}, "yandex.com": {}, "bk.ru": {},
	"list.ru": {}, "inbox.ru": {}, "rambler.ru": {}, "icloud.com": {},
	"protonmail.com": {}, "proton.me": {},
}

// VerifyEmail runs all checks on an email address and returns a scored result.
func VerifyEmail(email string) EmailResult {
	result := EmailResult{Email: strings.ToLower(strings.TrimSpace(email))}

	// 1. Syntax check
	result.IsValidSyntax = emailRegex.MatchString(result.Email)
	if !result.IsValidSyntax {
		result.Score = 0
		result.Status = "invalid"
		return result
	}

	parts := strings.SplitN(result.Email, "@", 2)
	domain := parts[1]

	// 2. Disposable check
	result.IsDisposable = IsDisposable(domain)

	// 3. Free provider check
	_, result.IsFreeProvider = freeProviders[domain]

	// 4. MX lookup
	mxRecords, err := net.LookupMX(domain)
	result.HasMX = err == nil && len(mxRecords) > 0
	if !result.HasMX {
		result.Score = 5
		result.Status = "invalid"
		return result
	}

	// 5. SMTP probe
	mxHost := mxRecords[0].Host
	result.SMTPValid, result.IsCatchAll, result.SMTPError = smtpProbe(mxHost, result.Email, domain)

	// 6. Score
	result.Score = calculateScore(result)
	result.Status = scoreToStatus(result.Score)
	return result
}

// smtpProbe connects to the MX server and checks if the email is deliverable.
// Also tests for catch-all by trying a random address.
func smtpProbe(mxHost, email, domain string) (valid bool, catchAll bool, errMsg string) {
	conn, err := net.DialTimeout("tcp", mxHost+":25", 10*time.Second)
	if err != nil {
		return false, false, fmt.Sprintf("connect: %v", err)
	}

	client, err := smtp.NewClient(conn, mxHost)
	if err != nil {
		conn.Close()
		return false, false, fmt.Sprintf("smtp client: %v", err)
	}
	defer client.Close()

	if err := client.Hello("verify.floq.app"); err != nil {
		return false, false, fmt.Sprintf("hello: %v", err)
	}
	if err := client.Mail("check@floq.app"); err != nil {
		return false, false, fmt.Sprintf("mail from: %v", err)
	}

	// Check real address
	err = client.Rcpt(email)
	if err != nil {
		return false, false, fmt.Sprintf("rcpt to: %v", err)
	}

	// Check catch-all with random address
	randomAddr := fmt.Sprintf("floq-verify-test-%d@%s", time.Now().UnixNano(), domain)
	catchAllErr := client.Rcpt(randomAddr)
	catchAll = catchAllErr == nil // if random address accepted -> catch-all

	client.Quit()
	return true, catchAll, ""
}

func calculateScore(r EmailResult) int {
	if r.IsDisposable {
		return 5
	}
	score := 0
	if r.IsValidSyntax {
		score += 20
	}
	if r.HasMX {
		score += 25
	}
	if r.SMTPValid {
		score += 40
	}
	if r.IsCatchAll {
		score -= 20
	}
	if r.IsFreeProvider {
		score -= 5
	}
	if score < 0 {
		score = 0
	}
	return score
}

func scoreToStatus(score int) string {
	if score >= 70 {
		return "valid"
	}
	if score >= 40 {
		return "risky"
	}
	return "invalid"
}
```

**Step 2: Commit**

```bash
git add backend/internal/verify/email.go
git commit -m "feat: add email verification with SMTP probe and scoring"
```

---

## Task 5: Telegram username verification

**Files:**
- Create: `backend/internal/verify/telegram.go`

**Step 1: Write TG verification**

```go
package verify

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramResult holds the result of Telegram username verification.
type TelegramResult struct {
	Username string `json:"username"`
	Exists   bool   `json:"exists"`
	Error    string `json:"error,omitempty"`
}

// VerifyTelegram checks if a Telegram username exists using the Bot API getChat.
func VerifyTelegram(bot *tgbotapi.BotAPI, username string) TelegramResult {
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	if username == "" {
		return TelegramResult{Username: username, Exists: false, Error: "empty username"}
	}

	chatCfg := tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{
			SuperGroupUsername: "@" + username,
		},
	}

	_, err := bot.GetChat(chatCfg)
	if err != nil {
		return TelegramResult{
			Username: username,
			Exists:   false,
			Error:    fmt.Sprintf("getChat: %v", err),
		}
	}

	return TelegramResult{Username: username, Exists: true}
}
```

**Step 2: Commit**

```bash
git add backend/internal/verify/telegram.go
git commit -m "feat: add Telegram username verification via Bot API"
```

---

## Task 6: Verification handler and usecase

**Files:**
- Create: `backend/internal/verify/handler.go`

**Step 1: Write the verification handler with 3 endpoints**

Endpoints:
- `POST /api/verify/email` — verify a single email (body: `{"email":"..."}`)
- `POST /api/verify/batch` — verify all unverified prospects for the user
- `GET /api/prospects/{id}/verify` — get verification result for a prospect

```go
package verify

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/daniil/floq/internal/prospects"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Handler struct {
	prospectRepo *prospects.Repository
	bot          *tgbotapi.BotAPI // nil if not configured
}

func RegisterRoutes(r chi.Router, prospectRepo *prospects.Repository, bot *tgbotapi.BotAPI) {
	h := &Handler{prospectRepo: prospectRepo, bot: bot}
	r.Post("/api/verify/email", h.verifyEmailSingle())
	r.Post("/api/verify/batch", h.verifyBatch())
	r.Get("/api/prospects/{id}/verify", h.getVerifyStatus())
}

func (h *Handler) verifyEmailSingle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
			writeError(w, http.StatusBadRequest, "email is required")
			return
		}
		result := VerifyEmail(body.Email)
		writeJSON(w, http.StatusOK, result)
	}
}

func (h *Handler) verifyBatch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := r.Context().Value("user_id").(uuid.UUID)

		allProspects, err := h.prospectRepo.ListProspects(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list prospects")
			return
		}

		var verified int
		for _, p := range allProspects {
			if p.VerifyStatus != "not_checked" {
				continue
			}

			// Verify email
			emailResult := VerifyEmail(p.Email)
			details, _ := json.Marshal(emailResult)

			// Verify TG if available
			var tgResult *TelegramResult
			if p.TelegramUsername != "" && h.bot != nil {
				res := VerifyTelegram(h.bot, p.TelegramUsername)
				tgResult = &res
			}

			// Merge TG into details
			if tgResult != nil {
				merged := map[string]any{}
				json.Unmarshal(details, &merged)
				merged["telegram"] = tgResult
				details, _ = json.Marshal(merged)
			}

			now := time.Now().UTC()
			if err := h.prospectRepo.UpdateVerification(r.Context(), p.ID, emailResult.Status, emailResult.Score, string(details), now); err != nil {
				continue // skip failed, don't stop batch
			}
			verified++
		}

		writeJSON(w, http.StatusOK, map[string]int{"verified": verified})
	}
}

func (h *Handler) getVerifyStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid prospect id")
			return
		}
		p, err := h.prospectRepo.GetProspect(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get prospect")
			return
		}
		if p == nil {
			writeError(w, http.StatusNotFound, "prospect not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"verify_status":  p.VerifyStatus,
			"verify_score":   p.VerifyScore,
			"verify_details": json.RawMessage(p.VerifyDetails),
			"verified_at":    p.VerifiedAt,
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
```

**Step 2: Commit**

```bash
git add backend/internal/verify/handler.go
git commit -m "feat: add verification API handler with single and batch endpoints"
```

---

## Task 7: Frontend — add verification API methods to api.ts

**Files:**
- Modify: `frontend/src/lib/api.ts`

**Step 1: Add Prospect and Verify types + API methods**

Add to the api object:
```typescript
// Prospects
getProspects: () => apiFetch<Prospect[]>("/api/prospects"),
createProspect: (data: CreateProspectBody) =>
  apiFetch<Prospect>("/api/prospects", {
    method: "POST",
    body: JSON.stringify(data),
  }),
deleteProspect: (id: string) =>
  apiFetch(`/api/prospects/${id}`, { method: "DELETE" }),

// Verification
verifyEmail: (email: string) =>
  apiFetch<EmailVerifyResult>("/api/verify/email", {
    method: "POST",
    body: JSON.stringify({ email }),
  }),
verifyBatch: () =>
  apiFetch<{ verified: number }>("/api/verify/batch", { method: "POST" }),
getVerifyStatus: (prospectId: string) =>
  apiFetch<VerifyStatus>(`/api/prospects/${prospectId}/verify`),
```

Add types:
```typescript
export interface Prospect {
  id: string;
  user_id: string;
  name: string;
  company: string;
  title: string;
  email: string;
  phone: string;
  telegram_username: string;
  industry: string;
  company_size: string;
  context: string;
  source: "manual" | "csv";
  status: "new" | "in_sequence" | "replied" | "converted" | "opted_out";
  verify_status: "not_checked" | "valid" | "risky" | "invalid";
  verify_score: number;
  verify_details: Record<string, any>;
  verified_at: string | null;
  converted_lead_id: string | null;
  created_at: string;
  updated_at: string;
}

export interface CreateProspectBody {
  name: string;
  company?: string;
  title?: string;
  email: string;
  phone?: string;
  telegram_username?: string;
  industry?: string;
  company_size?: string;
  context?: string;
}

export interface EmailVerifyResult {
  email: string;
  is_valid_syntax: boolean;
  has_mx: boolean;
  smtp_valid: boolean;
  smtp_error?: string;
  is_disposable: boolean;
  is_catch_all: boolean;
  is_free_provider: boolean;
  score: number;
  status: "valid" | "risky" | "invalid";
}

export interface VerifyStatus {
  verify_status: "not_checked" | "valid" | "risky" | "invalid";
  verify_score: number;
  verify_details: Record<string, any>;
  verified_at: string | null;
}
```

**Step 2: Commit**

```bash
git add frontend/src/lib/api.ts
git commit -m "feat: add prospect and verification API methods to frontend client"
```

---

## Task 8: Frontend — update prospects page with verification column

**Files:**
- Modify: `frontend/src/app/(dashboard)/prospects/page.tsx`

**Step 1: Add verification column to the prospects table**

Add a 5th column "Проверка" between "Email" and "Статус" with verification status icons:
- `not_checked` → gray dash icon, text "Не проверен"
- `valid` → green checkmark, text "Валидный" + score
- `risky` → yellow warning, text "Рискованный" + score
- `invalid` → red X, text "Невалидный" + score

**Step 2: Add "Проверить базу" button to the header**

Next to "Импорт CSV" button, add a shield/check button "Проверить базу" that calls `api.verifyBatch()`.

**Step 3: Update mock data to include verify fields**

Update MOCK_PROSPECTS to include verify_status, verify_score fields to preview the UI.

**Step 4: Add phone and telegram_username fields to the "Новый контакт" form**

Add two more inputs:
- Телефон (phone)
- Telegram username

**Step 5: Commit**

```bash
git add frontend/src/app/\(dashboard\)/prospects/page.tsx
git commit -m "feat: add verification status column and verify button to prospects page"
```

---

## Task 9: Build check and integration verification

**Step 1: Verify Go code compiles**

```bash
cd backend && go build ./...
```

Expected: clean build, no errors.

**Step 2: Verify frontend compiles**

```bash
cd frontend && npm run build
```

Expected: clean build.

**Step 3: Final commit if any fixes needed**

---

## Summary of created/modified files

### New files:
1. `backend/migrations/010_add_verification_fields.up.sql`
2. `backend/migrations/010_add_verification_fields.down.sql`
3. `backend/internal/verify/email.go`
4. `backend/internal/verify/telegram.go`
5. `backend/internal/verify/disposable.go`
6. `backend/internal/verify/handler.go`

### Modified files:
7. `backend/internal/prospects/repository.go` — new fields in struct + queries + UpdateVerification method
8. `backend/internal/prospects/handler.go` — accept new fields in create
9. `backend/internal/prospects/usecase.go` — handle new CSV columns
10. `frontend/src/lib/api.ts` — Prospect type + verify API methods
11. `frontend/src/app/(dashboard)/prospects/page.tsx` — verification column + verify button + new form fields
