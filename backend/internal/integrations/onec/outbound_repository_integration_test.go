//go:build integration

package onec_test

import (
	"context"
	"errors"
	"testing"

	"github.com/daniil/floq/internal/integrations/onec"
	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/daniil/floq/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_GetOutboundCredentials(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := onec.NewRepository(pool)
	ctx := context.Background()

	t.Run("active with base url", func(t *testing.T) {
		user := testutil.SeedUser(t, pool)
		_, err := pool.Exec(ctx, `
			INSERT INTO onec_credentials (user_id, base_url, auth_type, auth_secret, is_active)
			VALUES ($1, 'https://1c.example/odata/', 'token', 'tok-123', TRUE)`, user)
		require.NoError(t, err)

		creds, err := repo.GetOutboundCredentials(ctx, user)
		require.NoError(t, err)
		assert.Equal(t, "https://1c.example/odata", creds.BaseURL) // trailing slash trimmed by VO
		assert.Equal(t, domain.AuthTypeToken, creds.AuthType)
		assert.Equal(t, "tok-123", creds.AuthSecret)
	})

	t.Run("no row", func(t *testing.T) {
		user := testutil.SeedUser(t, pool)
		_, err := repo.GetOutboundCredentials(ctx, user)
		assert.True(t, errors.Is(err, onec.ErrOutboundNotConfigured), "got %v", err)
	})

	t.Run("inactive", func(t *testing.T) {
		user := testutil.SeedUser(t, pool)
		_, err := pool.Exec(ctx, `
			INSERT INTO onec_credentials (user_id, base_url, auth_type, is_active)
			VALUES ($1, 'https://1c.example', 'basic', FALSE)`, user)
		require.NoError(t, err)

		_, err = repo.GetOutboundCredentials(ctx, user)
		assert.True(t, errors.Is(err, onec.ErrOutboundNotConfigured), "inactive must not resolve; got %v", err)
	})

	t.Run("active but base url empty", func(t *testing.T) {
		user := testutil.SeedUser(t, pool)
		_, err := pool.Exec(ctx, `
			INSERT INTO onec_credentials (user_id, base_url, auth_type, is_active)
			VALUES ($1, '', 'basic', TRUE)`, user)
		require.NoError(t, err)

		_, err = repo.GetOutboundCredentials(ctx, user)
		assert.True(t, errors.Is(err, onec.ErrOutboundNotConfigured), "empty base_url is not usable; got %v", err)
	})
}

func TestRepository_OutboundRecord_UpsertAndExists(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := onec.NewRepository(pool)
	ctx := context.Background()
	user := testutil.SeedUser(t, pool)

	const extID, extType = "prospect:iv@ex.ru", "counterparty"

	// First push failed → error record. Not "processed" yet.
	errRec, err := domain.NewOutboundSyncRecord(user, extID, extType, domain.EventKindCounterpartyCreated, domain.SyncStatusError)
	require.NoError(t, err)
	require.NoError(t, repo.UpsertOutboundRecord(ctx, errRec))

	exists, err := repo.OutboundProcessedExists(ctx, user, extID, extType)
	require.NoError(t, err)
	assert.False(t, exists, "an error record must not count as already-processed")

	// Retry succeeded → upsert flips the same row to processed.
	okRec, err := domain.NewOutboundSyncRecord(user, extID, extType, domain.EventKindCounterpartyCreated, domain.SyncStatusProcessed)
	require.NoError(t, err)
	require.NoError(t, repo.UpsertOutboundRecord(ctx, okRec))

	exists, err = repo.OutboundProcessedExists(ctx, user, extID, extType)
	require.NoError(t, err)
	assert.True(t, exists, "after a successful push the record must read as processed")

	// Dedup: still exactly one row for the key.
	var count int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM onec_sync_records
		WHERE user_id = $1 AND external_id = $2 AND external_type = $3`,
		user, extID, extType).Scan(&count))
	assert.Equal(t, 1, count, "upsert must not create a duplicate row")
}
