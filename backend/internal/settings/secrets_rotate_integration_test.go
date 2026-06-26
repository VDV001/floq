//go:build integration

package settings_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/daniil/floq/internal/secrets"
	"github.com/daniil/floq/internal/settings"
	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// kek returns a base64-encoded 32-byte key whose every byte is b — a distinct,
// deterministic KEK per test (old vs new) without sharing testutil's key.
func kek(b byte) string {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = b
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// settingsSecretCols mirrors the encrypted user_settings columns (the same set
// guarded by migration 047). Kept here so the rotation tests pin EACH column.
var settingsSecretCols = []string{
	"telegram_bot_token",
	"imap_password",
	"resend_api_key",
	"smtp_password",
	"ai_api_key",
}

// newScratchAt47 reuses newScratchAt46 then migrates 047, leaving the
// post-drop schema (only the *_enc/*_nonce columns, no plaintext) — exactly the
// state in which KEK rotation operates.
func newScratchAt47(t *testing.T, suffix string) (*pgx.Conn, *migrate.Migrate, func()) {
	t.Helper()
	db, m, cleanup := newScratchAt46(t, suffix)
	require.NoError(t, m.Migrate(47))
	return db, m, cleanup
}

// seedEncrypted writes ciphertext sealed by `c` directly into <col>_enc/_nonce
// for the seeded owner — simulating a secret stored under the OLD KEK.
func seedEncrypted(t *testing.T, db *pgx.Conn, table, col string, c *secrets.Cipher, plaintext string) {
	t.Helper()
	ct, nonce, err := c.Encrypt(plaintext)
	require.NoError(t, err)
	_, err = db.Exec(context.Background(), fmt.Sprintf(
		`UPDATE %s SET %s_enc = $2, %s_nonce = $3 WHERE user_id = $1`, table, col, col),
		guardTestUserID, ct, nonce)
	require.NoError(t, err)
}

// TestRotateSecrets_ReencryptsEveryColumnUnderPrimary seeds each encrypted
// user_settings column under an OLD KEK, then rotates with a fallback cipher
// (primary=new, secondary=old) and proves EVERY column now decrypts under the
// new key alone. Table-driven per column: a regression that skips one column
// would otherwise hide behind the others (lesson from migration 047).
func TestRotateSecrets_ReencryptsEveryColumnUnderPrimary(t *testing.T) {
	ctx := context.Background()
	db, _, cleanup := newScratchAt47(t, "rotate")
	defer cleanup()

	oldC, err := secrets.NewCipher(kek(0xA1))
	require.NoError(t, err)
	newPrimary, err := secrets.NewCipher(kek(0xB2))
	require.NoError(t, err)
	fallback, err := secrets.NewCipherWithFallback(kek(0xB2), kek(0xA1))
	require.NoError(t, err)

	for _, col := range settingsSecretCols {
		seedEncrypted(t, db, "user_settings", col, oldC, "secret-"+col)
	}

	// Before rotation: none decrypt under the new key alone.
	ok, bad, err := settings.VerifySecretsKEK(ctx, db, newPrimary)
	require.NoError(t, err)
	assert.Equal(t, 0, ok)
	assert.Equal(t, len(settingsSecretCols), bad, "all old-key secrets must register as needing rotation")

	n, err := settings.RotateSecrets(ctx, db, fallback)
	require.NoError(t, err)
	assert.Equal(t, len(settingsSecretCols), n, "every non-empty secret re-encrypted")

	// After rotation: each column round-trips under the NEW key alone.
	for _, col := range settingsSecretCols {
		t.Run(col, func(t *testing.T) {
			assert.Equal(t, "secret-"+col, decryptColumn(t, db, "user_settings", col, newPrimary))
		})
	}

	ok, bad, err = settings.VerifySecretsKEK(ctx, db, newPrimary)
	require.NoError(t, err)
	assert.Equal(t, len(settingsSecretCols), ok)
	assert.Equal(t, 0, bad, "rotation complete — safe to drop FLOQ_SECRETS_KEK_OLD")

	// Convergent (safe to re-run): without a key-id marker every run re-encrypts
	// all rows, so the count stays N (not 0), and values still decrypt under
	// primary.
	n2, err := settings.RotateSecrets(ctx, db, fallback)
	require.NoError(t, err)
	assert.Equal(t, len(settingsSecretCols), n2, "re-run re-encrypts all rows (convergent, not no-op)")
	assert.Equal(t, "secret-imap_password", decryptColumn(t, db, "user_settings", "imap_password", newPrimary))
}

// TestRotateSecrets_SkipsEmptySecrets pins that unset secrets (no ciphertext)
// are never touched: rotation counts zero and verification sees zero rows.
func TestRotateSecrets_SkipsEmptySecrets(t *testing.T) {
	ctx := context.Background()
	db, _, cleanup := newScratchAt47(t, "rotateempty")
	defer cleanup()

	fallback, err := secrets.NewCipherWithFallback(kek(0xB2), kek(0xA1))
	require.NoError(t, err)
	primary, err := secrets.NewCipher(kek(0xB2))
	require.NoError(t, err)

	n, err := settings.RotateSecrets(ctx, db, fallback)
	require.NoError(t, err)
	assert.Equal(t, 0, n, "no ciphertext set → nothing to rotate")

	ok, bad, err := settings.VerifySecretsKEK(ctx, db, primary)
	require.NoError(t, err)
	assert.Equal(t, 0, ok)
	assert.Equal(t, 0, bad)
}
