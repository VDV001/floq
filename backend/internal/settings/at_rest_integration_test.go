//go:build integration

package settings_test

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/settings"
	"github.com/daniil/floq/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateSettings_EncryptsSecretAtRest(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	ctx := context.Background()
	cipher := backfillCipher(t)
	repo := settings.NewRepository(pool, cipher)

	// Pre-existing plaintext secret (simulates a row written before 037).
	_, err := pool.Exec(ctx,
		`INSERT INTO user_settings (user_id, imap_password) VALUES ($1, 'old-plaintext')`, userID)
	require.NoError(t, err)

	// Re-save the secret through the repository.
	require.NoError(t, repo.UpdateSettings(ctx, userID, map[string]any{
		"imap_password": "new-secret",
	}))

	// At rest: plaintext column blanked, ciphertext present and non-leaking.
	var plaintext string
	var enc, nonce []byte
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT imap_password, imap_password_enc, imap_password_nonce
		 FROM user_settings WHERE user_id = $1`, userID).
		Scan(&plaintext, &enc, &nonce))
	assert.Empty(t, plaintext, "old plaintext must be cleared on re-save")
	assert.NotEmpty(t, enc)
	assert.NotEmpty(t, nonce)
	assert.NotContains(t, string(enc), "new-secret")

	// Round-trips back through the decrypting read path.
	got, err := repo.GetSettings(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, "new-secret", got.IMAPPassword)
}
