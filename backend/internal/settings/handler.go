package settings

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
}

func RegisterRoutes(r chi.Router, pool *pgxpool.Pool) {
	h := &Handler{pool: pool}
	r.Get("/api/settings", h.getSettings())
	r.Put("/api/settings", h.updateSettings())
}

func getUserID(r *http.Request) uuid.UUID {
	uid, _ := r.Context().Value("user_id").(uuid.UUID)
	return uid
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
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
		userID := getUserID(r)
		if userID == uuid.Nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ctx := r.Context()

		// Get profile from users table.
		var fullName, email string
		err := h.pool.QueryRow(ctx,
			`SELECT full_name, email FROM users WHERE id = $1`, userID,
		).Scan(&fullName, &email)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load user profile")
			return
		}

		// Get settings (may not exist yet — use defaults).
		s := Settings{
			FullName:       fullName,
			Email:          email,
			IMAPPort:       "993",
			AIProvider:     "ollama",
			AIModel:        "gemma3:4b",
			NotifyTelegram: true,
		}

		err = h.pool.QueryRow(ctx,
			`SELECT telegram_bot_token, telegram_bot_active,
			        imap_host, imap_port, imap_user, imap_password,
			        resend_api_key,
			        ai_provider, ai_model, ai_api_key,
			        notify_telegram, notify_email_digest
			 FROM user_settings WHERE user_id = $1`, userID,
		).Scan(
			&s.TelegramBotToken, &s.TelegramBotActive,
			&s.IMAPHost, &s.IMAPPort, &s.IMAPUser, &s.IMAPPassword,
			&s.ResendAPIKey,
			&s.AIProvider, &s.AIModel, &s.AIAPIKey,
			&s.NotifyTelegram, &s.NotifyEmailDigest,
		)
		if err != nil && err.Error() != "no rows in result set" {
			writeError(w, http.StatusInternalServerError, "failed to load settings")
			return
		}

		// Mask sensitive fields.
		s.TelegramBotToken = maskSecret(s.TelegramBotToken)
		s.IMAPPassword = maskSecret(s.IMAPPassword)
		s.ResendAPIKey = maskSecret(s.ResendAPIKey)
		s.AIAPIKey = maskSecret(s.AIAPIKey)

		writeJSON(w, http.StatusOK, s)
	}
}

func (h *Handler) updateSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		if userID == uuid.Nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read request body")
			return
		}

		// Decode into a map so we know which fields were actually sent.
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		// Also decode into the struct for typed access.
		var input Settings
		if err := json.Unmarshal(body, &input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		// If telegram_bot_token is being set, validate it.
		if _, ok := raw["telegram_bot_token"]; ok && input.TelegramBotToken != "" {
			if err := validateTelegramToken(input.TelegramBotToken); err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid telegram bot token: %v", err))
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

		if len(cols) == 0 {
			writeError(w, http.StatusBadRequest, "no fields to update")
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
			writeError(w, http.StatusInternalServerError, "failed to save settings")
			return
		}

		// Return updated settings (reuse GET logic).
		var fullName, email string
		_ = h.pool.QueryRow(ctx,
			`SELECT full_name, email FROM users WHERE id = $1`, userID,
		).Scan(&fullName, &email)

		s := Settings{
			FullName:       fullName,
			Email:          email,
			IMAPPort:       "993",
			AIProvider:     "ollama",
			AIModel:        "gemma3:4b",
			NotifyTelegram: true,
		}

		_ = h.pool.QueryRow(ctx,
			`SELECT telegram_bot_token, telegram_bot_active,
			        imap_host, imap_port, imap_user, imap_password,
			        resend_api_key,
			        ai_provider, ai_model, ai_api_key,
			        notify_telegram, notify_email_digest
			 FROM user_settings WHERE user_id = $1`, userID,
		).Scan(
			&s.TelegramBotToken, &s.TelegramBotActive,
			&s.IMAPHost, &s.IMAPPort, &s.IMAPUser, &s.IMAPPassword,
			&s.ResendAPIKey,
			&s.AIProvider, &s.AIModel, &s.AIAPIKey,
			&s.NotifyTelegram, &s.NotifyEmailDigest,
		)

		// Mask sensitive fields.
		s.TelegramBotToken = maskSecret(s.TelegramBotToken)
		s.IMAPPassword = maskSecret(s.IMAPPassword)
		s.ResendAPIKey = maskSecret(s.ResendAPIKey)
		s.AIAPIKey = maskSecret(s.AIAPIKey)

		writeJSON(w, http.StatusOK, s)
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
