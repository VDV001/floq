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

// guardTestUserID is the single seeded owner used by the migration-047 tests.
const guardTestUserID = "11111111-1111-1111-1111-111111111111"

// baseDSN mirrors testutil's resolution (TEST_DATABASE_URL or the default
// localhost DSN) WITHOUT skipping — the drop-column safety net must run on a
// default-DSN integration pass, not silently no-op.
func baseDSN() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://floq:floq@localhost:5432/floq?sslmode=disable"
}

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

// newScratchAt46 spins up an isolated database migrated to version 046 (plaintext
// secret columns still present, one empty user_settings + onec_credentials row
// seeded for the owner) and returns a connection, the migrator, and a cleanup.
// The scratch DB name carries the pid + a per-test suffix so parallel runs and
// the two guard tests never collide.
func newScratchAt46(t *testing.T, suffix string) (*pgx.Conn, *migrate.Migrate, func()) {
	t.Helper()
	ctx := context.Background()
	base := baseDSN()
	scratch := fmt.Sprintf("floq_mig047_%s_%d", suffix, os.Getpid())

	admin, err := pgx.Connect(ctx, withDB(base, "postgres"))
	require.NoError(t, err)
	_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+scratch+" WITH (FORCE)")
	_, err = admin.Exec(ctx, "CREATE DATABASE "+scratch)
	require.NoError(t, err)

	scratchURL := withDB(base, scratch)
	m, err := migrate.New("file://../../migrations",
		strings.Replace(scratchURL, "postgres://", "pgx5://", 1))
	require.NoError(t, err)
	require.NoError(t, m.Migrate(46)) // plaintext columns still present

	db, err := pgx.Connect(ctx, scratchURL)
	require.NoError(t, err)
	_, err = db.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, 'g@t', 'h', 'G')`, guardTestUserID)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO user_settings (user_id) VALUES ($1)`, guardTestUserID)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `INSERT INTO onec_credentials (user_id) VALUES ($1)`, guardTestUserID)
	require.NoError(t, err)

	cleanup := func() {
		db.Close(context.Background())
		m.Close()
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+scratch+" WITH (FORCE)")
		admin.Close(context.Background())
	}
	return db, m, cleanup
}

// TestMigration047_GuardFiresPerSecretColumn pins EACH of the six secret
// predicates independently: with only that one column un-backfilled (plaintext
// set, ciphertext NULL), migration 047 must RAISE. A regression that drops or
// inverts a single predicate would otherwise slip through a test that only ever
// exercises one column, and the irreversible DROP would destroy that secret.
func TestMigration047_GuardFiresPerSecretColumn(t *testing.T) {
	ctx := context.Background()
	db, m, cleanup := newScratchAt46(t, "percol")
	defer cleanup()

	secretCols := []struct{ table, col string }{
		{"user_settings", "telegram_bot_token"},
		{"user_settings", "imap_password"},
		{"user_settings", "resend_api_key"},
		{"user_settings", "smtp_password"},
		{"user_settings", "ai_api_key"},
		{"onec_credentials", "auth_secret"},
	}

	for _, c := range secretCols {
		t.Run(c.col, func(t *testing.T) {
			// Un-backfill exactly this column; all others stay empty (no straggler).
			_, err := db.Exec(ctx, fmt.Sprintf(
				`UPDATE %s SET %s = 'straggler', %s_enc = NULL WHERE user_id = $1`, c.table, c.col, c.col),
				guardTestUserID)
			require.NoError(t, err)

			err = m.Migrate(47)
			require.Error(t, err, "047 must refuse to drop while %s is un-backfilled", c.col)
			assert.Contains(t, err.Error(), "un-backfilled")

			// Clear the dirty flag and the straggler so the next predicate is
			// tested in isolation.
			require.NoError(t, m.Force(46))
			_, err = db.Exec(ctx, fmt.Sprintf(
				`UPDATE %s SET %s = '' WHERE user_id = $1`, c.table, c.col), guardTestUserID)
			require.NoError(t, err)
		})
	}

	// With every secret empty (none un-backfilled), 047 applies cleanly.
	require.NoError(t, m.Migrate(47))
	assert.False(t, columnExists(t, db, "user_settings", "imap_password"))
	assert.False(t, columnExists(t, db, "onec_credentials", "auth_secret"))
}

// TestMigration047_RecoverWithRealBackfill drives the documented recovery with
// the real BackfillSecrets funcs (the same path `server -backfill-secrets`
// runs): it encrypts the stragglers, is idempotent and empty-skipping, lets 047
// apply, and the secrets round-trip.
func TestMigration047_RecoverWithRealBackfill(t *testing.T) {
	ctx := context.Background()
	db, m, cleanup := newScratchAt46(t, "recover")
	defer cleanup()

	// imap_password is a real secret; resend_api_key is left empty to pin the
	// empty-skip invariant (it must NOT be counted/encrypted).
	_, err := db.Exec(ctx,
		`UPDATE user_settings SET imap_password = 'imap-secret' WHERE user_id = $1`, guardTestUserID)
	require.NoError(t, err)
	_, err = db.Exec(ctx,
		`UPDATE onec_credentials SET auth_secret = '1c-secret' WHERE user_id = $1`, guardTestUserID)
	require.NoError(t, err)

	// Un-backfilled → guard refuses.
	require.Error(t, m.Migrate(47))
	require.NoError(t, m.Force(46))

	cipher := testutil.NewSecretCipher(t)
	nSettings, err := settings.BackfillSecrets(ctx, db, cipher)
	require.NoError(t, err)
	assert.Equal(t, 1, nSettings, "only the non-empty secret is encrypted (empty resend skipped)")
	nOnec, err := onec.BackfillSecrets(ctx, db, cipher)
	require.NoError(t, err)
	assert.Equal(t, 1, nOnec)

	// Idempotent: a second pass finds nothing left to encrypt.
	again, err := settings.BackfillSecrets(ctx, db, cipher)
	require.NoError(t, err)
	assert.Equal(t, 0, again, "backfill must be idempotent")
	againOnec, err := onec.BackfillSecrets(ctx, db, cipher)
	require.NoError(t, err)
	assert.Equal(t, 0, againOnec)

	require.NoError(t, m.Migrate(47), "047 applies once every secret is backfilled")

	assert.Equal(t, "imap-secret", decryptColumn(t, db, "user_settings", "imap_password", cipher))
	assert.Equal(t, "1c-secret", decryptColumn(t, db, "onec_credentials", "auth_secret", cipher))
	assert.False(t, columnExists(t, db, "user_settings", "imap_password"))
	assert.False(t, columnExists(t, db, "onec_credentials", "auth_secret"))
}

func columnExists(t *testing.T, db *pgx.Conn, table, column string) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns
		 WHERE table_name = $1 AND column_name = $2)`, table, column).Scan(&exists))
	return exists
}

func decryptColumn(t *testing.T, db *pgx.Conn, table, column string, cipher *secrets.Cipher) string {
	t.Helper()
	var enc, nonce []byte
	require.NoError(t, db.QueryRow(context.Background(), fmt.Sprintf(
		`SELECT %s_enc, %s_nonce FROM %s WHERE user_id = $1`, column, column, table), guardTestUserID).
		Scan(&enc, &nonce))
	got, err := cipher.Decrypt(enc, nonce)
	require.NoError(t, err)
	return got
}
