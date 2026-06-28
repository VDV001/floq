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

// PurgeTerminalDeliveriesOlderThan must delete only terminal (succeeded/failed)
// rows whose updated_at predates the threshold — recent terminal rows and
// pending rows of any age are spared (#212).
func TestRepository_PurgeTerminalDeliveriesOlderThan(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := webhooks.NewRepository(pool, testutil.NewSecretCipher(t))

	ep := mustEndpoint(t, userID, domain.EventLeadCreated)
	require.NoError(t, repo.CreateEndpoint(ctx, ep))

	mk := func(status domain.DeliveryStatus, updatedAt time.Time) uuid.UUID {
		d, err := domain.NewDelivery(userID, ep.ID, domain.EventLeadCreated, []byte(`{"x":1}`))
		require.NoError(t, err)
		require.NoError(t, repo.EnqueueDelivery(ctx, d))
		_, err = pool.Exec(ctx,
			`UPDATE webhook_deliveries SET status=$2, updated_at=$3 WHERE id=$1`,
			d.ID, string(status), updatedAt)
		require.NoError(t, err)
		return d.ID
	}

	now := time.Now().UTC()
	oldSucceeded := mk(domain.DeliverySucceeded, now.Add(-48*time.Hour))
	oldFailed := mk(domain.DeliveryFailed, now.Add(-48*time.Hour))
	recentSucceeded := mk(domain.DeliverySucceeded, now.Add(-1*time.Hour))
	oldPending := mk(domain.DeliveryPending, now.Add(-48*time.Hour))

	n, err := repo.PurgeTerminalDeliveriesOlderThan(ctx, now.Add(-24*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 2, n, "only old terminal (succeeded+failed) rows are purged")

	exists := func(id uuid.UUID) bool {
		var x int
		return pool.QueryRow(ctx, `SELECT 1 FROM webhook_deliveries WHERE id=$1`, id).Scan(&x) == nil
	}
	assert.False(t, exists(oldSucceeded), "old succeeded must be purged")
	assert.False(t, exists(oldFailed), "old failed must be purged")
	assert.True(t, exists(recentSucceeded), "recent terminal row is within the window, kept")
	assert.True(t, exists(oldPending), "pending rows are never purged regardless of age")
}
