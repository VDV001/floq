//go:build integration

package settings_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/daniil/floq/internal/secrets"
	"github.com/daniil/floq/internal/settings"
	"github.com/daniil/floq/internal/settings/domain"
	"github.com/daniil/floq/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// atRestCipher builds a real AES-256-GCM cipher so the at-rest path is
// exercised against actual crypto + a real database, not a stub.
func atRestCipher(t *testing.T) *secrets.Cipher {
	t.Helper()
	key := base64.StdEncoding.EncodeToString([]byte("settings-integration-test-kek-32"))
	c, err := secrets.NewCipher(key)
	require.NoError(t, err)
	return c
}

// TestUpdateSettings_EncryptsSecretAtRest verifies, for every secret column,
// that saving through the repository stores a non-leaking ciphertext in the
// *_enc/*_nonce columns and round-trips back to plaintext. The legacy plaintext
// columns were dropped in migration 047, so they are no longer written or read.
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
			repo := settings.NewRepository(pool, atRestCipher(t))

			// Save through the repository.
			require.NoError(t, repo.UpdateSettings(ctx, userID, map[string]any{tc.column: tc.value}))

			// At rest: ciphertext present and non-leaking (no plaintext column).
			var enc, nonce []byte
			require.NoError(t, pool.QueryRow(ctx, fmt.Sprintf(
				`SELECT %s_enc, %s_nonce FROM user_settings WHERE user_id = $1`,
				tc.column, tc.column), userID).
				Scan(&enc, &nonce))
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
