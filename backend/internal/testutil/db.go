//go:build integration

package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

const defaultDSN = "postgres://floq:floq@localhost:5432/floq?sslmode=disable"

// dsn returns the integration-test database connection string. It honours
// TEST_DATABASE_URL when set (e.g. to point at a postgres on an alternate
// port when 5432 is taken), falling back to the default local instance.
func dsn() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return defaultDSN
}

// TestDB returns a connection pool for integration tests.
// The pool is closed automatically when the test finishes.
func TestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn())
	require.NoError(t, err, "connect to test database")
	t.Cleanup(func() { pool.Close() })
	return pool
}

// SeedUser inserts a temporary user and schedules its deletion via t.Cleanup.
// Returns the user's UUID.
func SeedUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	userID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name) VALUES ($1, $2, $3, $4)`,
		userID, "test-"+userID.String()+"@example.com", "hash", "Test User")
	require.NoError(t, err, "seed test user")

	t.Cleanup(func() {
		// CASCADE will delete leads, prospects, sequences, sources, etc.
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	return userID
}
