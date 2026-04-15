package settings

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/daniil/floq/internal/settings/domain"
)

// Compile-time check.
var _ domain.TelegramTokenValidator = (*HTTPTelegramValidator)(nil)

// HTTPTelegramValidator validates Telegram bot tokens via the Telegram API.
type HTTPTelegramValidator struct {
	baseURL string // override for testing; empty = production URL
}

func (v *HTTPTelegramValidator) Validate(token string) error {
	base := v.baseURL
	if base == "" {
		base = "https://api.telegram.org"
	}
	resp, err := http.Get(fmt.Sprintf("%s/bot%s/getMe", base, token))
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
