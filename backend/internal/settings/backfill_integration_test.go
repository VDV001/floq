//go:build integration

package settings_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/daniil/floq/internal/secrets"
	"github.com/daniil/floq/internal/settings"
	"github.com/daniil/floq/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func backfillCipher(t *testing.T) *secrets.Cipher {
	t.Helper()
	key := base64.StdEncoding.EncodeToString([]byte("settings-backfill-test-kek-32byt"))
	c, err := secrets.NewCipher(key)
	require.NoError(t, err)
	return c
}

func TestBackfillSecrets_EncryptsLegacyPlaintext(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	ctx := context.Background()
	cipher := backfillCipher(t)

	// Simulate a pre-encryption row: plaintext secrets, enc columns NULL.
	_, err := pool.Exec(ctx, `
		INSERT INTO user_settings (user_id, imap_password, ai_api_key, resend_api_key)
		VALUES ($1, 'imap-plain', 'ai-plain', '')`, userID)
	require.NoError(t, err)

	n, err := settings.BackfillSecrets(ctx, pool, cipher)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "two non-empty secrets encrypted (empty resend skipped)")

	// enc columns now populated; reading back through the Store decrypts them.
	store := settings.NewStoreFromQuerier(pool, cipher)
	cfg, err := store.GetConfig(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, "imap-plain", cfg.IMAPPassword)
	assert.Equal(t, "ai-plain", cfg.AIAPIKey)

	// Idempotent: a second run finds nothing left to encrypt.
	n2, err := settings.BackfillSecrets(ctx, pool, cipher)
	require.NoError(t, err)
	assert.Equal(t, 0, n2)
}
