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

func mustEndpoint(t *testing.T, userID uuid.UUID, events ...domain.EventType) *domain.WebhookEndpoint {
	t.Helper()
	ep, err := domain.NewWebhookEndpoint(userID, "https://example.com/hook", events, "supersecretvalue123")
	require.NoError(t, err)
	return ep
}

func TestRepository_Endpoint_RoundTrip(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := webhooks.NewRepository(pool)

	ep := mustEndpoint(t, userID, domain.EventLeadCreated, domain.EventLeadQualified)
	require.NoError(t, repo.CreateEndpoint(ctx, ep))

	got, err := repo.ListEndpoints(ctx, userID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, ep.ID, got[0].ID)
	assert.Equal(t, "https://example.com/hook", got[0].URL.String())
	assert.Equal(t, "supersecretvalue123", got[0].Secret)
	assert.True(t, got[0].Active)
	assert.True(t, got[0].Subscribes(domain.EventLeadCreated))
	assert.True(t, got[0].Subscribes(domain.EventLeadQualified))
}

func TestRepository_GetEndpoint_FoundAndMissing(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := webhooks.NewRepository(pool)

	ep := mustEndpoint(t, userID, domain.EventLeadCreated)
	require.NoError(t, repo.CreateEndpoint(ctx, ep))

	_, found, err := repo.GetEndpoint(ctx, ep.ID)
	require.NoError(t, err)
	assert.True(t, found)

	_, found, err = repo.GetEndpoint(ctx, uuid.New())
	require.NoError(t, err)
	assert.False(t, found, "unknown ID must be not-found, not an error")
}

func TestRepository_DeleteEndpoint_CascadesDeliveries(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := webhooks.NewRepository(pool)

	ep := mustEndpoint(t, userID, domain.EventLeadCreated)
	require.NoError(t, repo.CreateEndpoint(ctx, ep))
	d, _ := domain.NewDelivery(userID, ep.ID, domain.EventLeadCreated, []byte(`{"x":1}`))
	require.NoError(t, repo.EnqueueDelivery(ctx, d))

	require.NoError(t, repo.DeleteEndpoint(ctx, ep.ID))

	// The delivery is gone too (ON DELETE CASCADE), so nothing is claimable.
	due, err := repo.ClaimDueDeliveries(ctx, 10, 3)
	require.NoError(t, err)
	for _, x := range due {
		assert.NotEqual(t, d.ID, x.ID, "delivery must cascade-delete with its endpoint")
	}
}

func TestRepository_Delivery_EnqueueClaimSave(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := webhooks.NewRepository(pool)

	ep := mustEndpoint(t, userID, domain.EventLeadCreated)
	require.NoError(t, repo.CreateEndpoint(ctx, ep))
	d, _ := domain.NewDelivery(userID, ep.ID, domain.EventLeadCreated, []byte(`{"event":"lead.created"}`))
	require.NoError(t, repo.EnqueueDelivery(ctx, d))

	// attempts=0 → due immediately.
	due, err := repo.ClaimDueDeliveries(ctx, 10, 3)
	require.NoError(t, err)
	require.True(t, containsDelivery(due, d.ID), "fresh pending delivery must be claimable")

	// Mark delivered and persist; it must no longer be claimable.
	claimed := findDelivery(due, d.ID)
	claimed.MarkDelivered(200, time.Now())
	require.NoError(t, repo.SaveDelivery(ctx, claimed))

	due, err = repo.ClaimDueDeliveries(ctx, 10, 3)
	require.NoError(t, err)
	assert.False(t, containsDelivery(due, d.ID), "succeeded delivery must not be re-claimed")
}

func TestRepository_ClaimDue_RespectsMaxAttempts(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := webhooks.NewRepository(pool)

	ep := mustEndpoint(t, userID, domain.EventLeadCreated)
	require.NoError(t, repo.CreateEndpoint(ctx, ep))
	d, _ := domain.NewDelivery(userID, ep.ID, domain.EventLeadCreated, []byte(`{}`))
	require.NoError(t, repo.EnqueueDelivery(ctx, d))

	// Exhaust attempts: 3 failures at maxAttempts=3 → terminal failed.
	now := time.Now()
	for i := 0; i < 3; i++ {
		d.MarkFailed("boom", 500, 3, now)
	}
	require.NoError(t, repo.SaveDelivery(ctx, d))

	due, err := repo.ClaimDueDeliveries(ctx, 10, 3)
	require.NoError(t, err)
	assert.False(t, containsDelivery(due, d.ID), "exhausted/failed delivery must not be claimed")
}

func TestRepository_ClaimDue_RespectsBackoff(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := webhooks.NewRepository(pool)

	ep := mustEndpoint(t, userID, domain.EventLeadCreated)
	require.NoError(t, repo.CreateEndpoint(ctx, ep))
	d, _ := domain.NewDelivery(userID, ep.ID, domain.EventLeadCreated, []byte(`{}`))
	require.NoError(t, repo.EnqueueDelivery(ctx, d))

	// One retryable failure → next_retry_at ~30s in the future (still pending,
	// attempts<max) → must NOT be claimable yet.
	d.MarkFailed("503", 503, 5, time.Now())
	require.NoError(t, repo.SaveDelivery(ctx, d))
	require.NotNil(t, d.NextRetryAt)

	due, err := repo.ClaimDueDeliveries(ctx, 10, 5)
	require.NoError(t, err)
	assert.False(t, containsDelivery(due, d.ID),
		"a delivery whose backoff has not elapsed must not be claimed")
}

func containsDelivery(ds []*domain.WebhookDelivery, id uuid.UUID) bool {
	return findDelivery(ds, id) != nil
}

func findDelivery(ds []*domain.WebhookDelivery, id uuid.UUID) *domain.WebhookDelivery {
	for _, d := range ds {
		if d.ID == id {
			return d
		}
	}
	return nil
}
