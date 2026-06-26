//go:build integration

package settings_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/daniil/floq/internal/integrations/onec"
	"github.com/daniil/floq/internal/secrets"
	"github.com/daniil/floq/internal/settings"
	"github.com/daniil/floq/internal/testutil"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withDB rewrites only the database segment of a postgres DSN
// (scheme://user:pass@host:port/<db>[?params]), leaving the authority intact.
// A naive strings.Replace of "/"+dbname would corrupt the URL when the username
// equals the dbname (e.g. the repo default postgres://floq:floq@host/floq).
func withDB(base, db string) string {
	dsn, query := base, ""
	if i := strings.IndexByte(base, '?'); i >= 0 {
		dsn, query = base[:i], base[i:]
	}
	slash := strings.LastIndexByte(dsn, '/')
	return dsn[:slash+1] + db + query
}

// TestMigration047_GuardRefusesUnbackfilledThenRecovers exercises migration
// 047's data-loss guard end to end against a throwaway database, for BOTH
// secret tables:
//   1. an un-backfilled secret (plaintext set, ciphertext NULL) makes 047 RAISE,
//   2. golang-migrate leaves the schema dirty at v47,
//   3. the documented recovery — `force 46`, run the real BackfillSecrets,
//      re-apply — lets 047 complete, drops the plaintext columns, and the
//      backfilled secrets round-trip.
// This pins the only safety net between a missed backfill and the irreversible
// DROP COLUMN, which manual verification alone would leave open to a future
// regression (renamed column, inverted predicate).
func TestMigration047_GuardRefusesUnbackfilledThenRecovers(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL not set; migration guard test needs a real pg")
	}
	ctx := context.Background()

	// Scratch DB name carries the pid so parallel `go test` invocations against
	// the same Postgres do not collide on the shared name.
	scratch := fmt.Sprintf("floq_mig047_guard_%d", os.Getpid())
	adminURL := withDB(base, "postgres")
	scratchURL := withDB(base, scratch)

	// Isolated database so neither the guard failure (which dirties the schema)
	// nor the DROP touches the shared integration DB.
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

	db, err := pgx.Connect(ctx, scratchURL)
	require.NoError(t, err)
	defer db.Close(context.Background())

	// Seed un-backfilled secrets in BOTH tables: plaintext set, ciphertext NULL.
	userID := "11111111-1111-1111-1111-111111111111"
	_, err = db.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, 'g@t', 'h', 'G')`, userID)
	require.NoError(t, err)
	_, err = db.Exec(ctx,
		`INSERT INTO user_settings (user_id, imap_password) VALUES ($1, 'imap-secret')`, userID)
	require.NoError(t, err)
	_, err = db.Exec(ctx,
		`INSERT INTO onec_credentials (user_id, auth_secret) VALUES ($1, '1c-secret')`, userID)
	require.NoError(t, err)

	// 047 must REFUSE: the guard RAISEs and leaves the schema dirty at v47.
	err = m.Migrate(47)
	require.Error(t, err, "047 must refuse to drop while a secret is un-backfilled")
	assert.Contains(t, err.Error(), "un-backfilled", "error must explain why the drop was refused")
	assert.True(t, columnExists(t, db, "user_settings", "imap_password"),
		"plaintext column must survive a refused drop")

	// Recovery: clear the dirty flag, encrypt the stragglers via the real
	// backfill funcs (the same code path `server -backfill-secrets` runs), and
	// re-apply.
	require.NoError(t, m.Force(46))
	cipher := testutil.NewSecretCipher(t)
	nSettings, err := settings.BackfillSecrets(ctx, db, cipher)
	require.NoError(t, err)
	assert.Equal(t, 1, nSettings, "settings backfill must encrypt the one straggler")
	nOnec, err := onec.BackfillSecrets(ctx, db, cipher)
	require.NoError(t, err)
	assert.Equal(t, 1, nOnec, "onec backfill must encrypt the one straggler")

	require.NoError(t, m.Migrate(47), "047 must apply once every secret is backfilled")

	// Both backfilled secrets decrypt back to their original plaintext.
	assert.Equal(t, "imap-secret", decryptColumn(t, db, "user_settings", "imap_password", userID, cipher))
	assert.Equal(t, "1c-secret", decryptColumn(t, db, "onec_credentials", "auth_secret", userID, cipher))

	assert.False(t, columnExists(t, db, "user_settings", "imap_password"),
		"plaintext column must be dropped after a clean apply")
	assert.False(t, columnExists(t, db, "onec_credentials", "auth_secret"),
		"onec plaintext column must be dropped after a clean apply")
}

func columnExists(t *testing.T, db *pgx.Conn, table, column string) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns
		 WHERE table_name = $1 AND column_name = $2)`, table, column).Scan(&exists))
	return exists
}

func decryptColumn(t *testing.T, db *pgx.Conn, table, column, userID string, cipher *secrets.Cipher) string {
	t.Helper()
	var enc, nonce []byte
	require.NoError(t, db.QueryRow(context.Background(), fmt.Sprintf(
		`SELECT %s_enc, %s_nonce FROM %s WHERE user_id = $1`, column, column, table), userID).
		Scan(&enc, &nonce))
	got, err := cipher.Decrypt(enc, nonce)
	require.NoError(t, err)
	return got
}
