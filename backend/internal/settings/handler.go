package settings

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/settings/domain"
	"github.com/go-chi/chi/v5"
)

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

	// AI
	AIProvider string `json:"ai_provider"`
	AIModel    string `json:"ai_model"`
	AIAPIKey   string `json:"ai_api_key"`

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
}

type Handler struct {
	uc *UseCase
}

func RegisterRoutes(r chi.Router, uc *UseCase) {
	h := &Handler{uc: uc}
	r.Get("/api/settings", h.getSettings())
	r.Put("/api/settings", h.updateSettings())
	r.Post("/api/settings/test-imap", h.testIMAP())
}

// maskSecret returns the last 4 characters of a secret prefixed with "...",
// or an empty string if the input is empty.
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 4 {
		return "..." + s
	}
	return "..." + s[len(s)-4:]
}

// domainToDTO converts a domain.Settings to the JSON DTO.
func domainToDTO(ds *domain.Settings) Settings {
	return Settings{
		FullName:           ds.FullName,
		Email:              ds.Email,
		TelegramBotToken:   ds.TelegramBotToken,
		TelegramBotActive:  ds.TelegramBotActive,
		IMAPHost:           ds.IMAPHost,
		IMAPPort:           ds.IMAPPort,
		IMAPUser:           ds.IMAPUser,
		IMAPPassword:       ds.IMAPPassword,
		ResendAPIKey:       ds.ResendAPIKey,
		AIProvider:         ds.AIProvider,
		AIModel:            ds.AIModel,
		AIAPIKey:           ds.AIAPIKey,
		NotifyTelegram:     ds.NotifyTelegram,
		NotifyEmailDigest:  ds.NotifyEmailDigest,
		AutoQualify:        ds.AutoQualify,
		AutoDraft:          ds.AutoDraft,
		AutoSend:           ds.AutoSend,
		AutoSendDelayMin:   ds.AutoSendDelayMin,
		AutoFollowup:       ds.AutoFollowup,
		AutoFollowupDays:   ds.AutoFollowupDays,
		AutoProspectToLead: ds.AutoProspectToLead,
		AutoVerifyImport:   ds.AutoVerifyImport,
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

		ds, err := h.uc.UpdateSettings(r.Context(), userID, body)
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

		// Try TLS connection to IMAP server
		addr := net.JoinHostPort(body.Host, body.Port)
		conn, err := tls.DialWithDialer(
			&net.Dialer{Timeout: 10 * time.Second},
			"tcp", addr,
			&tls.Config{ServerName: body.Host},
		)
		if err != nil {
			httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": fmt.Sprintf("Не удалось подключиться: %v", err)})
			return
		}
		defer conn.Close()

		// Read server greeting
		buf := make([]byte, 1024)
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": "Сервер не отвечает"})
			return
		}
		greeting := string(buf[:n])

		// Send LOGIN command with quoted strings
		loginCmd := fmt.Sprintf("A1 LOGIN \"%s\" \"%s\"\r\n", body.User, body.Password)
		_, err = conn.Write([]byte(loginCmd))
		if err != nil {
			httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": "Ошибка отправки команды"})
			return
		}

		// Read response (may be multiple lines)
		var response string
		for i := 0; i < 5; i++ {
			_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			n, err = conn.Read(buf)
			if err != nil {
				break
			}
			response += string(buf[:n])
			if strings.Contains(response, "A1 OK") || strings.Contains(response, "A1 NO") || strings.Contains(response, "A1 BAD") {
				break
			}
		}

		// Send LOGOUT
		_, _ = conn.Write([]byte("A2 LOGOUT\r\n"))

		_ = greeting // used for connection check

		if strings.Contains(response, "A1 OK") {
			httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "message": "Подключение успешно!"})
		} else {
			httputil.WriteJSON(w, http.StatusOK, map[string]any{"success": false, "error": "Неверный логин или пароль"})
		}
	}
}
