//go:build integration

package webhooks_test

import (
	"context"
	"testing"
	"time"

	"github.com/daniil/floq/internal/testutil"
	"github.com/daniil/floq/internal/webhooks"
	"github.com/daniil/floq/internal/webhooks/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #212 part 2: the delivery claim must lease the row it takes so a second
// concurrent worker skips it, hand rows out earliest-due-first, and reclaim a
// row whose lease has expired (crashed worker).
func TestRepository_ClaimOneDelivery_LeasesOrdersReclaims(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := webhooks.NewRepository(pool, testutil.NewSecretCipher(t))

	ep := mustEndpoint(t, userID, domain.EventLeadCreated)
	require.NoError(t, repo.CreateEndpoint(ctx, ep))

	now := time.Now().UTC()
	mk := func(dueOffset time.Duration) uuid.UUID {
		d, err := domain.NewDelivery(userID, ep.ID, domain.EventLeadCreated, []byte(`{"x":1}`))
		require.NoError(t, err)
		require.NoError(t, repo.EnqueueDelivery(ctx, d))
		due := now.Add(dueOffset)
		_, err = pool.Exec(ctx, `UPDATE webhook_deliveries SET next_retry_at=$2 WHERE id=$1`, d.ID, due)
		require.NoError(t, err)
		return d.ID
	}

	early := mk(-270 * time.Second)
	mid := mk(-70 * time.Second)

	const lease = 300

	got1, err := repo.ClaimDueDelivery(ctx, 5, lease)
	require.NoError(t, err)
	require.NotNil(t, got1)
	assert.Equal(t, early, got1.ID, "first claim takes the earliest-due delivery")

	got2, err := repo.ClaimDueDelivery(ctx, 5, lease)
	require.NoError(t, err)
	require.NotNil(t, got2)
	assert.Equal(t, mid, got2.ID, "second claim skips the leased 'early' and takes 'mid'")

	got3, err := repo.ClaimDueDelivery(ctx, 5, lease)
	require.NoError(t, err)
	assert.Nil(t, got3, "both due rows are leased — nothing claimable")

	// Expire the lease on 'early' (worker died mid-delivery) → reclaimable.
	_, err = pool.Exec(ctx,
		`UPDATE webhook_deliveries SET locked_until = now() - interval '1 second' WHERE id = $1`, early)
	require.NoError(t, err)

	got4, err := repo.ClaimDueDelivery(ctx, 5, lease)
	require.NoError(t, err)
	require.NotNil(t, got4, "an expired lease must be reclaimable")
	assert.Equal(t, early, got4.ID)
}

// Saving an outcome releases the lease so next_retry_at alone governs the retry.
func TestRepository_SaveDelivery_ClearsLease(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := webhooks.NewRepository(pool, testutil.NewSecretCipher(t))

	ep := mustEndpoint(t, userID, domain.EventLeadCreated)
	require.NoError(t, repo.CreateEndpoint(ctx, ep))

	d, err := domain.NewDelivery(userID, ep.ID, domain.EventLeadCreated, []byte(`{"x":1}`))
	require.NoError(t, err)
	require.NoError(t, repo.EnqueueDelivery(ctx, d))

	got, err := repo.ClaimDueDelivery(ctx, 5, 300)
	require.NoError(t, err)
	require.NotNil(t, got)

	got.MarkFailed("transient", 0, got.Attempts+1, time.Now().UTC()) // back to pending + backoff
	require.NoError(t, repo.SaveDelivery(ctx, got))

	var lockedUntil *time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT locked_until FROM webhook_deliveries WHERE id = $1`, d.ID).Scan(&lockedUntil))
	assert.Nil(t, lockedUntil, "SaveDelivery must clear the lease")
}
