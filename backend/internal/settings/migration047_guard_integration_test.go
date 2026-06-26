//go:build integration

package settings_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/daniil/floq/internal/settings"
	"github.com/daniil/floq/internal/testutil"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigration047_GuardRefusesUnbackfilledThenRecovers exercises migration
// 047's data-loss guard end to end against a throwaway database:
//   1. an un-backfilled secret (plaintext set, ciphertext NULL) makes 047 RAISE,
//   2. golang-migrate leaves the schema dirty at v47,
//   3. the documented recovery — `force 46`, encrypt the straggler, re-apply —
//      lets 047 complete and drops the plaintext column.
// This pins the only safety net standing between a missed backfill and the
// irreversible DROP COLUMN, which manual verification alone would leave
// unguarded against a future regression (renamed column, inverted predicate).
func TestMigration047_GuardRefusesUnbackfilledThenRecovers(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL not set; migration guard test needs a real pg")
	}
	ctx := context.Background()

	cfg, err := pgx.ParseConfig(base)
	require.NoError(t, err)
	const scratch = "floq_mig047_guard_test"
	adminURL := strings.Replace(base, "/"+cfg.Database, "/postgres", 1)
	scratchURL := strings.Replace(base, "/"+cfg.Database, "/"+scratch, 1)

	// Create an isolated database so neither the guard failure (which dirties
	// the schema) nor the DROP touches the shared integration DB.
	admin, err := pgx.Connect(ctx, adminURL)
	require.NoError(t, err)
	_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+scratch+" WITH (FORCE)")
	_, err = admin.Exec(ctx, "CREATE DATABASE "+scratch)
	require.NoError(t, err)
	defer func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+scratch+" WITH (FORCE)")
		admin.Close(context.Background())
	}()

	m, err := migrate.New("file://../../migrations",
		strings.Replace(scratchURL, "postgres://", "pgx5://", 1))
	require.NoError(t, err)
	defer m.Close()

	// Migrate up to 046 — plaintext columns still present.
	require.NoError(t, m.Migrate(46))

	// Seed an un-backfilled secret: plaintext set, ciphertext NULL.
	db, err := pgx.Connect(ctx, scratchURL)
	require.NoError(t, err)
	userID := "11111111-1111-1111-1111-111111111111"
	_, err = db.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, 'g@t', 'h', 'G')`, userID)
	require.NoError(t, err)
	_, err = db.Exec(ctx,
		`INSERT INTO user_settings (user_id, imap_password) VALUES ($1, 'plaintext-secret')`, userID)
	require.NoError(t, err)

	// 047 must REFUSE: the guard RAISEs and leaves the schema dirty at v47.
	err = m.Migrate(47)
	require.Error(t, err, "047 must refuse to drop while a secret is un-backfilled")
	assert.Contains(t, err.Error(), "un-backfilled", "error must explain why the drop was refused")

	var stillThere bool
	require.NoError(t, db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns
		 WHERE table_name = 'user_settings' AND column_name = 'imap_password')`).Scan(&stillThere))
	assert.True(t, stillThere, "plaintext column must survive a refused drop")

	// Recovery: clear the dirty flag, encrypt the straggler via the real
	// backfill (the same code path the server runs pre-migration), re-apply.
	require.NoError(t, m.Force(46))
	cipher := testutil.NewSecretCipher(t)
	n, err := settings.BackfillSecrets(ctx, db, cipher)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "backfill must encrypt exactly the one straggler secret")
	require.NoError(t, m.Migrate(47), "047 must apply once every secret is backfilled")

	// The backfilled secret decrypts back to its original plaintext.
	var enc, nonce []byte
	require.NoError(t, db.QueryRow(ctx,
		`SELECT imap_password_enc, imap_password_nonce FROM user_settings WHERE user_id = $1`, userID).
		Scan(&enc, &nonce))
	got, err := cipher.Decrypt(enc, nonce)
	require.NoError(t, err)
	assert.Equal(t, "plaintext-secret", got, "backfilled ciphertext must round-trip")

	require.NoError(t, db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns
		 WHERE table_name = 'user_settings' AND column_name = 'imap_password')`).Scan(&stillThere))
	assert.False(t, stillThere, "plaintext column must be dropped after a clean apply")
	db.Close(ctx)
}
