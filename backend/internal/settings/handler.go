package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/settings/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// smtpErrorToUserMessage maps a typed SMTP error from the composition
// root's tester into a Russian user-facing message. Owning this mapping
// here (rather than in cmd/server/helpers.go) is the Clean Architecture
// rule: UI strings live next to the handler that emits them, not in the
// infrastructure that produces the technical error.
func smtpErrorToUserMessage(err error) string {
	switch {
	case errors.Is(err, ErrSMTPProxyDial):
		return "Не удалось подключиться через прокси"
	case errors.Is(err, ErrSMTPDial):
		return "Не удалось подключиться"
	case errors.Is(err, ErrSMTPClient):
		return "Ошибка создания SMTP-клиента"
	case errors.Is(err, ErrSMTPStartTLS):
		return "Ошибка STARTTLS"
	case errors.Is(err, ErrSMTPAuth):
		return "Неверный логин или пароль SMTP"
	default:
		return "Ошибка SMTP"
	}
}

// aiErrorToUserMessage maps a typed AI connection-test error into a
// Russian user-facing message. Owning this here keeps UI copy next to
// the handler; the composition-root tester returns only typed sentinels.
// The default preserves the generic "connection error" wrapping for
// cloud providers whose raw SDK error is already informative enough.
func aiErrorToUserMessage(err error) string {
	switch {
	case errors.Is(err, ErrAIModelNotFound):
		return "Модель не найдена в Ollama. Скачайте её командой «ollama pull <модель>» и проверьте, что имя указано верно."
	case errors.Is(err, ErrAIAuth):
		return "API-ключ отклонён. Проверьте, что ключ верный и активен."
	case errors.Is(err, ErrAIRateLimit):
		return "Слишком много запросов к провайдеру. Подождите и попробуйте ещё раз."
	case errors.Is(err, ErrAIUnreachable):
		return "Не удалось подключиться к Ollama. Проверьте, что сервер запущен и адрес указан верно."
	case errors.Is(err, ErrAIUnknownProvider):
		return "Неизвестный провайдер ИИ"
	default:
		return fmt.Sprintf("Ошибка подключения: %v", err)
	}
}

// resendErrorToUserMessage mirrors smtpErrorToUserMessage for Resend.
func resendErrorToUserMessage(err error) string {
	switch {
	case errors.Is(err, ErrResendAuth):
		return "Неверный API ключ Resend"
	case errors.Is(err, ErrResendRequest):
		return "Ошибка запроса"
	default:
		return "Ошибка Resend"
	}
}

// Settings is the JSON DTO returned by the API.
type Settings struct {
	// Profile (read-only, from users table)
	FullName string `json:"full_name"`
	Email    string `json:"email"`

	// Telegram
	TelegramBotToken  string `json:"telegram_bot_token"`
	TelegramBotActive bool   `json:"telegram_bot_active"`

	// IMAP
	IMAPHost     string `json:"imap_host"`
	IMAPPort     string `json:"imap_port"`
	IMAPUser     string `json:"imap_user"`
	IMAPPassword string `json:"imap_password"`

	// Resend
	ResendAPIKey string `json:"resend_api_key"`

	// SMTP
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     string `json:"smtp_port"`
	SMTPUser     string `json:"smtp_user"`
	SMTPPassword string `json:"smtp_password"`
	SMTPActive   bool   `json:"smtp_active"`

	// AI
	AIProvider          string `json:"ai_provider"`
	AIModel             string `json:"ai_model"`
	AIAPIKey            string `json:"ai_api_key"`
	AIStyleCheckEnabled bool   `json:"ai_style_check_enabled"`

	// Connection statuses (computed, read-only)
	IMAPActive   bool `json:"imap_active"`
	ResendActive bool `json:"resend_active"`
	AIActive     bool `json:"ai_active"`

	// Notifications
	NotifyTelegram    bool `json:"notify_telegram"`
	NotifyEmailDigest bool `json:"notify_email_digest"`

	// Automations
	AutoQualify        bool `json:"auto_qualify"`
	AutoDraft          bool `json:"auto_draft"`
	AutoSend           bool `json:"auto_send"`
	AutoSendDelayMin   int  `json:"auto_send_delay_min"`
	AutoFollowup       bool `json:"auto_followup"`
	AutoFollowupDays   int  `json:"auto_followup_days"`
	AutoProspectToLead bool `json:"auto_prospect_to_lead"`
	AutoVerifyImport   bool `json:"auto_verify_import"`

	// Inbox view preference — see domain.Settings.AggregatedInboxView.
	AggregatedInboxView bool `json:"aggregated_inbox_view"`
}

// AITester tests an AI provider connection by sending a simple prompt.
// Injected from main.go to avoid import cycles with ai/providers.
type AITester func(ctx context.Context, provider, model, apiKey string) (providerName string, err error)

// SMTPTester tests SMTP connection and authentication.
// Injected from main.go to keep infrastructure logic out of the handler.
type SMTPTester func(ctx context.Context, host, port, user, password string) error

// ResendTester verifies a Resend API key.
// Injected from main.go to keep infrastructure logic out of the handler.
type ResendTester func(ctx context.Context, apiKey string) error

// UsageCounter counts leads for usage stats.
// Injected from main.go to avoid circular imports with leads package.
type UsageCounter func(ctx context.Context, userID uuid.UUID) (monthLeads, totalLeads int, err error)

type Handler struct {
	uc           *UseCase
	aiTester     AITester
	smtpTester   SMTPTester
	resendTester ResendTester
	usageCounter UsageCounter
}

func RegisterRoutes(r chi.Router, uc *UseCase, aiTester AITester, smtpTester SMTPTester, resendTester ResendTester, usageCounter UsageCounter) {
	h := &Handler{uc: uc, aiTester: aiTester, smtpTester: smtpTester, resendTester: resendTester, usageCounter: usageCounter}
	r.Get("/api/settings", h.getSettings())
	r.Put("/api/settings", h.updateSettings())
	r.Post("/api/settings/test-imap", h.testIMAP())
	r.Post("/api/settings/test-ai", h.testAI())
	r.Post("/api/settings/test-resend", h.testResend())
	r.Post("/api/settings/test-smtp", h.testSMTP())
	r.Get("/api/usage", h.getUsage())
}

// maskSecret reveals only the last 4 characters of a secret so the read API
// never exposes a usable credential. A secret too short to mask meaningfully
// (≤4 chars) is replaced wholesale rather than leaked verbatim (matches the
// onec package's copy).
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 4 {
		return "••••"
	}
	return "..." + s[len(s)-4:]
}

// domainToDTO converts a domain.Settings to the JSON DTO.
func domainToDTO(ds *domain.Settings) Settings {
	aiActive := ds.AIProvider == "ollama" || (ds.AIProvider != "" && ds.AIAPIKey != "")

	return Settings{
		FullName: ds.FullName,
		Email:    ds.Email,
		// Secrets are masked here, at the presentation boundary — the usecase
		// returns them raw.
		TelegramBotToken:    maskSecret(ds.TelegramBotToken),
		TelegramBotActive:   ds.TelegramBotActive,
		IMAPHost:            ds.IMAPHost,
		IMAPPort:            ds.IMAPPort,
		IMAPUser:            ds.IMAPUser,
		IMAPPassword:        maskSecret(ds.IMAPPassword),
		ResendAPIKey:        maskSecret(ds.ResendAPIKey),
		SMTPHost:            ds.SMTPHost,
		SMTPPort:            ds.SMTPPort,
		SMTPUser:            ds.SMTPUser,
		SMTPPassword:        maskSecret(ds.SMTPPassword),
		SMTPActive:          ds.SMTPHost != "" && ds.SMTPUser != "" && ds.SMTPPassword != "",
		AIProvider:          ds.AIProvider,
		AIModel:             ds.AIModel,
		AIAPIKey:            maskSecret(ds.AIAPIKey),
		AIStyleCheckEnabled: ds.AIStyleCheckEnabled,
		IMAPActive:          ds.IMAPHost != "" && ds.IMAPUser != "" && ds.IMAPPassword != "",
		ResendActive:        ds.ResendAPIKey != "",
		AIActive:            aiActive,
		NotifyTelegram:      ds.NotifyTelegram,
		NotifyEmailDigest:   ds.NotifyEmailDigest,
		AutoQualify:         ds.AutoQualify,
		AutoDraft:           ds.AutoDraft,
		AutoSend:            ds.AutoSend,
		AutoSendDelayMin:    ds.AutoSendDelayMin,
		AutoFollowup:        ds.AutoFollowup,
		AutoFollowupDays:    ds.AutoFollowupDays,
		AutoProspectToLead:  ds.AutoProspectToLead,
		AutoVerifyImport:    ds.AutoVerifyImport,
		AggregatedInboxView: ds.AggregatedInboxView,
	}
}

func (h *Handler) getSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ds, err := h.uc.GetSettings(r.Context(), userID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to load settings")
			return
		}

		httputil.WriteJSON(w, http.StatusOK, domainToDTO(ds))
	}
}

func (h *Handler) updateSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "failed to read request body")
			return
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		var input SettingsInput
		if err := json.Unmarshal(body, &input); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		ds, err := h.uc.UpdateSettings(r.Context(), userID, raw, input)
		if err != nil {
			msg := err.Error()
			if strings.Contains(msg, "invalid") || strings.Contains(msg, "no fields") {
				httputil.WriteError(w, http.StatusBadRequest, msg)
				return
			}
			httputil.WriteError(w, http.StatusInternalServerError, "failed to save settings")
			return
		}

		httputil.WriteJSON(w, http.StatusOK, domainToDTO(ds))
	}
}

func (h *Handler) testIMAP() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var body struct {
			Host      string `json:"host"`
			Port      string `json:"port"`
			User      string `json:"user"`
			Password  string `json:"password"`
			UseStored bool   `json:"use_stored"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		// If use_stored, read password from DB via use case
		if body.UseStored || body.Password == "" {
			storedPwd, err := h.uc.GetStoredIMAPPassword(r.Context(), userID)
			if err == nil && storedPwd != "" {
				body.Password = storedPwd
			}
		}

		if body.Host == "" || body.Port == "" || body.User == "" || body.Password == "" {
			httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": "Заполните все поля IMAP"})
			return
		}

		tester := &IMAPTester{}
		if err := tester.TestConnection(body.Host, body.Port, body.User, body.Password); err != nil {
			httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": err.Error()})
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "message": "Подключение успешно!"})
	}
}

func (h *Handler) testAI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var body struct {
			Provider  string `json:"provider"`
			Model     string `json:"model"`
			APIKey    string `json:"api_key"`
			UseStored bool   `json:"use_stored"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		// If use_stored or empty key, read from DB
		if body.UseStored || body.APIKey == "" {
			stored, err := h.uc.GetStoredAISettings(r.Context(), userID)
			if err == nil {
				if body.Provider == "" {
					body.Provider = stored.Provider
				}
				if body.Model == "" {
					body.Model = stored.Model
				}
				if body.APIKey == "" {
					body.APIKey = stored.APIKey
				}
			}
		}

		if body.Provider == "" {
			httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": "Провайдер не выбран"})
			return
		}

		if h.aiTester == nil {
			httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": "Тест AI недоступен"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		providerName, err := h.aiTester(ctx, body.Provider, body.Model, body.APIKey)
		if err != nil {
			httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": aiErrorToUserMessage(err)})
			return
		}

		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"success":  true,
			"message":  fmt.Sprintf("Подключение к %s успешно!", providerName),
			"provider": providerName,
		})
	}
}

func (h *Handler) testResend() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var body struct {
			APIKey    string `json:"api_key"`
			UseStored bool   `json:"use_stored"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		if body.UseStored || body.APIKey == "" {
			stored, err := h.uc.GetStoredResendKey(r.Context(), userID)
			if err == nil && stored != "" {
				body.APIKey = stored
			}
		}

		if body.APIKey == "" {
			httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": "API ключ не задан"})
			return
		}

		if h.resendTester == nil {
			httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": "Тест Resend недоступен"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		if err := h.resendTester(ctx, body.APIKey); err != nil {
			httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": resendErrorToUserMessage(err)})
			return
		}

		httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "message": "Resend API подключен!"})
	}
}

func (h *Handler) testSMTP() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Host     string `json:"host"`
			Port     string `json:"port"`
			User     string `json:"user"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		if body.Host == "" || body.User == "" || body.Password == "" {
			httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": "Заполните хост, пользователя и пароль SMTP"})
			return
		}
		if body.Port == "" {
			body.Port = "465"
		}

		if h.smtpTester == nil {
			httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": "Тест SMTP недоступен"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		if err := h.smtpTester(ctx, body.Host, body.Port, body.User, body.Password); err != nil {
			httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": smtpErrorToUserMessage(err)})
			return
		}

		httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "message": "SMTP подключен!"})
	}
}

func (h *Handler) getUsage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		monthLeads, totalLeads := 0, 0
		if h.usageCounter != nil {
			var err error
			monthLeads, totalLeads, err = h.usageCounter(r.Context(), userID)
			if err != nil {
				httputil.WriteError(w, http.StatusInternalServerError, "failed to get usage")
				return
			}
		}

		// Plan limits (from env or default). Will be dynamic when billing is added.
		plan := "growth"
		limit := 1000

		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"plan":        plan,
			"limit":       limit,
			"month_leads": monthLeads,
			"total_leads": totalLeads,
		})
	}
}
