//go:build integration

package settings_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/daniil/floq/internal/settings"
	"github.com/daniil/floq/internal/settings/domain"
	"github.com/daniil/floq/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateSettings_EncryptsSecretAtRest verifies, for every secret column,
// that re-saving through the repository clears the legacy plaintext column,
// stores a non-leaking ciphertext, and round-trips back to plaintext.
func TestUpdateSettings_EncryptsSecretAtRest(t *testing.T) {
	cases := []struct {
		column string
		value  string
		read   func(*domain.Settings) string
	}{
		{"imap_password", "imap-secret-xyz", func(s *domain.Settings) string { return s.IMAPPassword }},
		{"telegram_bot_token", "123:tg-secret-xyz", func(s *domain.Settings) string { return s.TelegramBotToken }},
		{"resend_api_key", "re_secret_xyz", func(s *domain.Settings) string { return s.ResendAPIKey }},
		{"smtp_password", "smtp-secret-xyz", func(s *domain.Settings) string { return s.SMTPPassword }},
		{"ai_api_key", "sk-ai-secret-xyz", func(s *domain.Settings) string { return s.AIAPIKey }},
	}

	for _, tc := range cases {
		t.Run(tc.column, func(t *testing.T) {
			pool := testutil.TestDB(t)
			userID := testutil.SeedUser(t, pool)
			ctx := context.Background()
			repo := settings.NewRepository(pool, backfillCipher(t))

			// Pre-existing plaintext secret (simulates a row written before 037).
			_, err := pool.Exec(ctx, fmt.Sprintf(
				`INSERT INTO user_settings (user_id, %s) VALUES ($1, 'old-plaintext')`, tc.column), userID)
			require.NoError(t, err)

			// Re-save through the repository.
			require.NoError(t, repo.UpdateSettings(ctx, userID, map[string]any{tc.column: tc.value}))

			// At rest: plaintext blanked, ciphertext present and non-leaking.
			var plaintext string
			var enc, nonce []byte
			require.NoError(t, pool.QueryRow(ctx, fmt.Sprintf(
				`SELECT %s, %s_enc, %s_nonce FROM user_settings WHERE user_id = $1`,
				tc.column, tc.column, tc.column), userID).
				Scan(&plaintext, &enc, &nonce))
			assert.Empty(t, plaintext, "old plaintext must be cleared on re-save")
			assert.NotEmpty(t, enc)
			assert.NotEmpty(t, nonce)
			assert.False(t, strings.Contains(string(enc), tc.value), "ciphertext must not leak the secret")

			// Round-trips back through the decrypting read path.
			got, err := repo.GetSettings(ctx, userID)
			require.NoError(t, err)
			assert.Equal(t, tc.value, tc.read(got))
		})
	}
}
