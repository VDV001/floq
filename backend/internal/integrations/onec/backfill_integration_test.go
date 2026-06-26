//go:build integration

package onec_test

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/integrations/onec"
	"github.com/daniil/floq/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackfillSecrets_EncryptsLegacyAuthSecret(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	ctx := context.Background()
	cipher := testCipher(t)

	// Pre-encryption row: plaintext auth_secret, enc column NULL.
	_, err := pool.Exec(ctx, `
		INSERT INTO onec_credentials (user_id, base_url, auth_type, auth_secret, is_active)
		VALUES ($1, 'https://1c.example.com', 'token', 'legacy-secret', TRUE)`, userID)
	require.NoError(t, err)

	n, err := onec.BackfillSecrets(ctx, pool, cipher)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	// Reads back through the decrypting repo.
	repo := onec.NewRepository(pool, cipher)
	out, err := repo.GetOutboundCredentials(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, "legacy-secret", out.AuthSecret)

	// Idempotent.
	n2, err := onec.BackfillSecrets(ctx, pool, cipher)
	require.NoError(t, err)
	assert.Equal(t, 0, n2)
}
