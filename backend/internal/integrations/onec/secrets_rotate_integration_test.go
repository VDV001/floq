//go:build integration

package onec_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/daniil/floq/internal/integrations/onec"
	"github.com/daniil/floq/internal/secrets"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rotateTestUserID = "22222222-2222-2222-2222-222222222222"

// kek returns a base64-encoded 32-byte key whose every byte is b.
func kek(b byte) string {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = b
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// baseDSN/withDB mirror the settings rotation tests: the rotation pass scans
// the WHOLE onec_credentials table, so it must run on an isolated scratch DB
// (a shared base DB would carry foreign rows encrypted under another KEK that
// the rotation cipher cannot decrypt).
func baseDSN() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://floq:floq@localhost:5432/floq?sslmode=disable"
}

func withDB(base, db string) string {
	dsn, query := base, ""
	if i := strings.IndexByte(base, '?'); i >= 0 {
		dsn, query = base[:i], base[i:]
	}
	slash := strings.LastIndexByte(dsn, '/')
	return dsn[:slash+1] + db + query
}

// newScratchAt47 spins up an isolated DB migrated to 047 (post-drop schema) with
// one seeded user + empty onec_credentials row, and returns a conn + cleanup.
func newScratchAt47(t *testing.T, suffix string) (*pgx.Conn, func()) {
	t.Helper()
	ctx := context.Background()
	base := baseDSN()
	scratch := fmt.Sprintf("floq_onecrot_%s_%d", suffix, os.Getpid())

	admin, err := pgx.Connect(ctx, withDB(base, "postgres"))
	require.NoError(t, err)
	_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+scratch+" WITH (FORCE)")
	_, err = admin.Exec(ctx, "CREATE DATABASE "+scratch)
	require.NoError(t, err)

	scratchURL := withDB(base, scratch)
	m, err := migrate.New("file://../../../migrations",
		strings.Replace(scratchURL, "postgres://", "pgx5://", 1))
	require.NoError(t, err)
	require.NoError(t, m.Migrate(47))

	db, err := pgx.Connect(ctx, scratchURL)
	require.NoError(t, err)
	_, err = db.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, 'r@t', 'h', 'R')`, rotateTestUserID)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO onec_credentials (user_id) VALUES ($1)`, rotateTestUserID)
	require.NoError(t, err)

	cleanup := func() {
		db.Close(context.Background())
		m.Close()
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+scratch+" WITH (FORCE)")
		admin.Close(context.Background())
	}
	return db, cleanup
}

func TestRotateSecrets_OnecReencryptsAuthSecretUnderPrimary(t *testing.T) {
	ctx := context.Background()
	db, cleanup := newScratchAt47(t, "rotate")
	defer cleanup()

	oldC, err := secrets.NewCipher(kek(0xA1))
	require.NoError(t, err)
	newPrimary, err := secrets.NewCipher(kek(0xB2))
	require.NoError(t, err)
	fallback, err := secrets.NewCipherWithFallback(kek(0xB2), kek(0xA1))
	require.NoError(t, err)

	// Seed auth_secret encrypted under the OLD KEK.
	ct, nonce, err := oldC.Encrypt("1c-topsecret")
	require.NoError(t, err)
	_, err = db.Exec(ctx,
		`UPDATE onec_credentials SET auth_secret_enc = $2, auth_secret_nonce = $3 WHERE user_id = $1`,
		rotateTestUserID, ct, nonce)
	require.NoError(t, err)

	// Before: needs rotation under the new key.
	ok, bad, err := onec.VerifySecretsKEK(ctx, db, newPrimary)
	require.NoError(t, err)
	assert.Equal(t, 0, ok)
	assert.Equal(t, 1, bad)

	n, err := onec.RotateSecrets(ctx, db, fallback)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	// After: decrypts under the new key alone.
	var encAfter, nonceAfter []byte
	require.NoError(t, db.QueryRow(ctx,
		`SELECT auth_secret_enc, auth_secret_nonce FROM onec_credentials WHERE user_id = $1`, rotateTestUserID).
		Scan(&encAfter, &nonceAfter))
	got, err := newPrimary.Decrypt(encAfter, nonceAfter)
	require.NoError(t, err)
	assert.Equal(t, "1c-topsecret", got)

	ok, bad, err = onec.VerifySecretsKEK(ctx, db, newPrimary)
	require.NoError(t, err)
	assert.Equal(t, 1, ok)
	assert.Equal(t, 0, bad, "rotation complete — safe to drop FLOQ_SECRETS_KEK_OLD")

	// Convergent: re-run re-encrypts the row again (count 1, not 0) and it still
	// decrypts under primary.
	n2, err := onec.RotateSecrets(ctx, db, fallback)
	require.NoError(t, err)
	assert.Equal(t, 1, n2)
}

func TestRotateSecrets_OnecSkipsEmptySecret(t *testing.T) {
	ctx := context.Background()
	db, cleanup := newScratchAt47(t, "rotateempty")
	defer cleanup()

	fallback, err := secrets.NewCipherWithFallback(kek(0xB2), kek(0xA1))
	require.NoError(t, err)
	primary, err := secrets.NewCipher(kek(0xB2))
	require.NoError(t, err)

	n, err := onec.RotateSecrets(ctx, db, fallback)
	require.NoError(t, err)
	assert.Equal(t, 0, n, "no ciphertext set → nothing to rotate")

	ok, bad, err := onec.VerifySecretsKEK(ctx, db, primary)
	require.NoError(t, err)
	assert.Equal(t, 0, ok)
	assert.Equal(t, 0, bad)
}
