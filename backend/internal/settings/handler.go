package settings

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/daniil/floq/internal/httputil"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	pool *pgxpool.Pool
}

type Settings struct {
	// Profile (read-only, from users table)
	FullName string `json:"full_name"`
	Email    string `json:"email"`

	// Telegram
	TelegramBotToken string `json:"telegram_bot_token"`
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

func RegisterRoutes(r chi.Router, pool *pgxpool.Pool) {
	h := &Handler{pool: pool}
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

func (h *Handler) getSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ctx := r.Context()

		// Get profile from users table.
		var fullName, email string
		err := h.pool.QueryRow(ctx,
			`SELECT full_name, email FROM users WHERE id = $1`, userID,
		).Scan(&fullName, &email)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to load user profile")
			return
		}

		// Get settings (may not exist yet — use defaults).
		s := Settings{
			FullName:           fullName,
			Email:              email,
			IMAPPort:           "993",
			AIProvider:         "ollama",
			AIModel:            "gemma3:4b",
			NotifyTelegram:     true,
			AutoQualify:        true,
			AutoDraft:          true,
			AutoFollowup:       true,
			AutoFollowupDays:   2,
			AutoSendDelayMin:   5,
			AutoProspectToLead: true,
		}

		err = h.pool.QueryRow(ctx,
			`SELECT telegram_bot_token, telegram_bot_active,
			        imap_host, imap_port, imap_user, imap_password,
			        resend_api_key,
			        ai_provider, ai_model, ai_api_key,
			        notify_telegram, notify_email_digest,
			        auto_qualify, auto_draft, auto_send, auto_send_delay_min,
			        auto_followup, auto_followup_days, auto_prospect_to_lead, auto_verify_import
			 FROM user_settings WHERE user_id = $1`, userID,
		).Scan(
			&s.TelegramBotToken, &s.TelegramBotActive,
			&s.IMAPHost, &s.IMAPPort, &s.IMAPUser, &s.IMAPPassword,
			&s.ResendAPIKey,
			&s.AIProvider, &s.AIModel, &s.AIAPIKey,
			&s.NotifyTelegram, &s.NotifyEmailDigest,
			&s.AutoQualify, &s.AutoDraft, &s.AutoSend, &s.AutoSendDelayMin,
			&s.AutoFollowup, &s.AutoFollowupDays, &s.AutoProspectToLead, &s.AutoVerifyImport,
		)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to load settings")
			return
		}

		// Mask sensitive fields.
		s.TelegramBotToken = maskSecret(s.TelegramBotToken)
		s.IMAPPassword = maskSecret(s.IMAPPassword)
		s.ResendAPIKey = maskSecret(s.ResendAPIKey)
		s.AIAPIKey = maskSecret(s.AIAPIKey)

		httputil.WriteJSON(w, http.StatusOK, s)
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

		// Decode into a map so we know which fields were actually sent.
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		// Also decode into the struct for typed access.
		var input Settings
		if err := json.Unmarshal(body, &input); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		// If telegram_bot_token is being set, validate it.
		if _, ok := raw["telegram_bot_token"]; ok && input.TelegramBotToken != "" {
			if err := validateTelegramToken(input.TelegramBotToken); err != nil {
				httputil.WriteError(w, http.StatusBadRequest, fmt.Sprintf("invalid telegram bot token: %v", err))
				return
			}
		}

		ctx := r.Context()

		// Build the UPSERT dynamically based on which fields are present.
		// We always set updated_at.
		type col struct {
			name string
			val  any
		}
		var cols []col

		if _, ok := raw["telegram_bot_token"]; ok {
			cols = append(cols, col{"telegram_bot_token", input.TelegramBotToken})
			// Auto-activate if token is valid (validation passed above)
			if input.TelegramBotToken != "" {
				cols = append(cols, col{"telegram_bot_active", true})
			} else {
				cols = append(cols, col{"telegram_bot_active", false})
			}
		} else if _, ok := raw["telegram_bot_active"]; ok {
			cols = append(cols, col{"telegram_bot_active", input.TelegramBotActive})
		}
		if _, ok := raw["imap_host"]; ok {
			cols = append(cols, col{"imap_host", input.IMAPHost})
		}
		if _, ok := raw["imap_port"]; ok {
			cols = append(cols, col{"imap_port", input.IMAPPort})
		}
		if _, ok := raw["imap_user"]; ok {
			cols = append(cols, col{"imap_user", input.IMAPUser})
		}
		if _, ok := raw["imap_password"]; ok {
			cols = append(cols, col{"imap_password", input.IMAPPassword})
		}
		if _, ok := raw["resend_api_key"]; ok {
			cols = append(cols, col{"resend_api_key", input.ResendAPIKey})
		}
		if _, ok := raw["ai_provider"]; ok {
			cols = append(cols, col{"ai_provider", input.AIProvider})
		}
		if _, ok := raw["ai_model"]; ok {
			cols = append(cols, col{"ai_model", input.AIModel})
		}
		if _, ok := raw["ai_api_key"]; ok {
			cols = append(cols, col{"ai_api_key", input.AIAPIKey})
		}
		if _, ok := raw["notify_telegram"]; ok {
			cols = append(cols, col{"notify_telegram", input.NotifyTelegram})
		}
		if _, ok := raw["notify_email_digest"]; ok {
			cols = append(cols, col{"notify_email_digest", input.NotifyEmailDigest})
		}
		if _, ok := raw["auto_qualify"]; ok {
			cols = append(cols, col{"auto_qualify", input.AutoQualify})
		}
		if _, ok := raw["auto_draft"]; ok {
			cols = append(cols, col{"auto_draft", input.AutoDraft})
		}
		if _, ok := raw["auto_send"]; ok {
			cols = append(cols, col{"auto_send", input.AutoSend})
		}
		if _, ok := raw["auto_send_delay_min"]; ok {
			cols = append(cols, col{"auto_send_delay_min", input.AutoSendDelayMin})
		}
		if _, ok := raw["auto_followup"]; ok {
			cols = append(cols, col{"auto_followup", input.AutoFollowup})
		}
		if _, ok := raw["auto_followup_days"]; ok {
			cols = append(cols, col{"auto_followup_days", input.AutoFollowupDays})
		}
		if _, ok := raw["auto_prospect_to_lead"]; ok {
			cols = append(cols, col{"auto_prospect_to_lead", input.AutoProspectToLead})
		}
		if _, ok := raw["auto_verify_import"]; ok {
			cols = append(cols, col{"auto_verify_import", input.AutoVerifyImport})
		}

		if len(cols) == 0 {
			httputil.WriteError(w, http.StatusBadRequest, "no fields to update")
			return
		}

		// Build SQL: INSERT ... ON CONFLICT DO UPDATE SET ...
		// $1 = user_id, then $2..$N for column values.
		insertCols := "user_id"
		insertVals := "$1"
		updateSet := "updated_at = NOW()"
		args := []any{userID}

		for i, c := range cols {
			paramIdx := i + 2 // $2, $3, ...
			insertCols += fmt.Sprintf(", %s", c.name)
			insertVals += fmt.Sprintf(", $%d", paramIdx)
			updateSet += fmt.Sprintf(", %s = $%d", c.name, paramIdx)
			args = append(args, c.val)
		}

		query := fmt.Sprintf(
			`INSERT INTO user_settings (%s) VALUES (%s)
			 ON CONFLICT (user_id) DO UPDATE SET %s`,
			insertCols, insertVals, updateSet,
		)

		_, err = h.pool.Exec(ctx, query, args...)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to save settings")
			return
		}

		// Return updated settings (reuse GET logic).
		var fullName, email string
		_ = h.pool.QueryRow(ctx,
			`SELECT full_name, email FROM users WHERE id = $1`, userID,
		).Scan(&fullName, &email)

		s := Settings{
			FullName:           fullName,
			Email:              email,
			IMAPPort:           "993",
			AIProvider:         "ollama",
			AIModel:            "gemma3:4b",
			NotifyTelegram:     true,
			AutoQualify:        true,
			AutoDraft:          true,
			AutoFollowup:       true,
			AutoFollowupDays:   2,
			AutoSendDelayMin:   5,
			AutoProspectToLead: true,
		}

		_ = h.pool.QueryRow(ctx,
			`SELECT telegram_bot_token, telegram_bot_active,
			        imap_host, imap_port, imap_user, imap_password,
			        resend_api_key,
			        ai_provider, ai_model, ai_api_key,
			        notify_telegram, notify_email_digest,
			        auto_qualify, auto_draft, auto_send, auto_send_delay_min,
			        auto_followup, auto_followup_days, auto_prospect_to_lead, auto_verify_import
			 FROM user_settings WHERE user_id = $1`, userID,
		).Scan(
			&s.TelegramBotToken, &s.TelegramBotActive,
			&s.IMAPHost, &s.IMAPPort, &s.IMAPUser, &s.IMAPPassword,
			&s.ResendAPIKey,
			&s.AIProvider, &s.AIModel, &s.AIAPIKey,
			&s.NotifyTelegram, &s.NotifyEmailDigest,
			&s.AutoQualify, &s.AutoDraft, &s.AutoSend, &s.AutoSendDelayMin,
			&s.AutoFollowup, &s.AutoFollowupDays, &s.AutoProspectToLead, &s.AutoVerifyImport,
		)

		// Mask sensitive fields.
		s.TelegramBotToken = maskSecret(s.TelegramBotToken)
		s.IMAPPassword = maskSecret(s.IMAPPassword)
		s.ResendAPIKey = maskSecret(s.ResendAPIKey)
		s.AIAPIKey = maskSecret(s.AIAPIKey)

		httputil.WriteJSON(w, http.StatusOK, s)
	}
}

// validateTelegramToken calls the Telegram getMe API to verify the token is valid.
func validateTelegramToken(token string) error {
	resp, err := http.Get(fmt.Sprintf("https://api.telegram.org/bot%s/getMe", token))
	if err != nil {
		return fmt.Errorf("failed to reach Telegram API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Telegram API returned status %d", resp.StatusCode)
	}

	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode Telegram response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("Telegram API returned ok=false")
	}
	return nil
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

		// If use_stored, read password from DB
		if body.UseStored || body.Password == "" {
			ctx := r.Context()
			var storedPwd string
			err := h.pool.QueryRow(ctx,
				`SELECT imap_password FROM user_settings WHERE user_id = $1`, userID,
			).Scan(&storedPwd)
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
