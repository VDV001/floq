//go:build integration

package onec_test

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/integrations/onec"
	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/daniil/floq/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustConfig(t *testing.T, baseURL string, at domain.AuthType, secret, webhook string, active bool) *domain.CredentialsConfig {
	t.Helper()
	c, err := domain.NewCredentialsConfig(baseURL, at, secret, webhook, active)
	require.NoError(t, err)
	return c
}

func TestRepository_CredentialsConfig_NotFoundThenInsertThenUpdate(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := onec.NewRepository(pool, testCipher(t))
	ctx := context.Background()

	// No row yet → found=false, no error.
	_, found, err := repo.GetCredentialsConfig(ctx, userID)
	require.NoError(t, err)
	assert.False(t, found, "fresh user has no config")

	// Insert.
	require.NoError(t, repo.UpsertCredentialsConfig(ctx, userID,
		mustConfig(t, "https://1c.example.com", domain.AuthTypeToken, "sek", "", false)))

	got, found, err := repo.GetCredentialsConfig(ctx, userID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "https://1c.example.com", got.BaseURL)
	assert.Equal(t, domain.AuthTypeToken, got.AuthType)
	assert.Equal(t, "sek", got.AuthSecret)
	assert.False(t, got.IsActive)

	// Update a subset (flip active, change secret) — upsert merges in place.
	require.NoError(t, repo.UpsertCredentialsConfig(ctx, userID,
		mustConfig(t, "https://1c.example.com", domain.AuthTypeToken, "sek2", "", true)))

	got2, _, err := repo.GetCredentialsConfig(ctx, userID)
	require.NoError(t, err)
	assert.True(t, got2.IsActive)
	assert.Equal(t, "sek2", got2.AuthSecret)
}

func TestRepository_CredentialsConfig_EncryptsAuthSecretAtRest(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := onec.NewRepository(pool, testCipher(t))
	ctx := context.Background()

	require.NoError(t, repo.UpsertCredentialsConfig(ctx, userID,
		mustConfig(t, "https://1c.example.com", domain.AuthTypeToken, "topsecret", "", true)))

	// At rest: ciphertext present in the byte columns (the plaintext
	// auth_secret column was dropped in migration 047) and the ciphertext does
	// not leak the secret.
	var enc, nonce []byte
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT auth_secret_enc, auth_secret_nonce
		 FROM onec_credentials WHERE user_id = $1`, userID).
		Scan(&enc, &nonce))
	assert.NotEmpty(t, enc, "ciphertext must be stored")
	assert.NotEmpty(t, nonce, "nonce must be stored")
	assert.NotContains(t, string(enc), "topsecret")

	// Round-trips back to plaintext through both read paths.
	cfg, _, err := repo.GetCredentialsConfig(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, "topsecret", cfg.AuthSecret)

	out, err := repo.GetOutboundCredentials(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, "topsecret", out.AuthSecret)
}

func TestRepository_GetCredentialsConfig_ReturnsInactiveBlankBaseURL(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := onec.NewRepository(pool, testCipher(t))
	ctx := context.Background()

	// An inactive, blank-base-url row is a real saved state (user filled the
	// secret first). GetOutboundCredentials would skip it; the config reader
	// must still return it so the UI can show what's there.
	require.NoError(t, repo.UpsertCredentialsConfig(ctx, userID,
		mustConfig(t, "", domain.AuthTypeBasic, "draftsecret", "", false)))

	got, found, err := repo.GetCredentialsConfig(ctx, userID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "", got.BaseURL)
	assert.Equal(t, "draftsecret", got.AuthSecret)
	assert.False(t, got.IsActive)
}
