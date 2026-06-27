//go:build integration

package webhooks_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/daniil/floq/internal/db"
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
	repo := webhooks.NewRepository(pool, testutil.NewSecretCipher(t))

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

// The signing secret must be encrypted at rest: a raw read of the stored bytes
// must not contain the plaintext, and the round-trip must still recover it.
func TestRepository_Secret_EncryptedAtRest(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := webhooks.NewRepository(pool, testutil.NewSecretCipher(t))

	ep := mustEndpoint(t, userID, domain.EventLeadCreated)
	require.NoError(t, repo.CreateEndpoint(ctx, ep))

	var ciphertext []byte
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT secret_ciphertext FROM webhook_endpoints WHERE id = $1`, ep.ID).Scan(&ciphertext))
	require.NotEmpty(t, ciphertext)
	assert.NotContains(t, string(ciphertext), "supersecretvalue123",
		"plaintext secret must never be stored")

	// And it decrypts back on read.
	got, found, err := repo.GetEndpoint(ctx, ep.ID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "supersecretvalue123", got.Secret)
}

func TestRepository_GetEndpoint_FoundAndMissing(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := webhooks.NewRepository(pool, testutil.NewSecretCipher(t))

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
	repo := webhooks.NewRepository(pool, testutil.NewSecretCipher(t))

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
	repo := webhooks.NewRepository(pool, testutil.NewSecretCipher(t))

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
	repo := webhooks.NewRepository(pool, testutil.NewSecretCipher(t))

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
	repo := webhooks.NewRepository(pool, testutil.NewSecretCipher(t))

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

func TestRepository_SetEndpointActive_RoundTrip(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := webhooks.NewRepository(pool, testutil.NewSecretCipher(t))

	ep := mustEndpoint(t, userID, domain.EventLeadCreated)
	require.NoError(t, repo.CreateEndpoint(ctx, ep))

	require.NoError(t, repo.SetEndpointActive(ctx, ep.ID, false))
	got, found, err := repo.GetEndpoint(ctx, ep.ID)
	require.NoError(t, err)
	require.True(t, found)
	assert.False(t, got.Active, "endpoint must be persisted as inactive")

	require.NoError(t, repo.SetEndpointActive(ctx, ep.ID, true))
	got, _, err = repo.GetEndpoint(ctx, ep.ID)
	require.NoError(t, err)
	assert.True(t, got.Active, "endpoint must be persisted as active again")
}

func TestRepository_ClaimDue_OrdersByEffectiveDueTime(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := webhooks.NewRepository(pool, testutil.NewSecretCipher(t))

	ep := mustEndpoint(t, userID, domain.EventLeadCreated)
	require.NoError(t, repo.CreateEndpoint(ctx, ep))

	// Three due deliveries with distinct effective due-times. The worker must
	// claim them earliest-due first — by next_retry_at, treating NULL (never
	// attempted) as due-since-creation — so retries fire in schedule order
	// regardless of when each row was last updated (updated_at).
	mkFailed := func(payload string, failedAt time.Time) *domain.WebhookDelivery {
		d, err := domain.NewDelivery(userID, ep.ID, domain.EventLeadCreated, []byte(payload))
		require.NoError(t, err)
		require.NoError(t, repo.EnqueueDelivery(ctx, d))
		// One retryable failure scheduled in the past → past-due now; its
		// next_retry_at = failedAt + base backoff (30s).
		d.MarkFailed("503", 503, 5, failedAt)
		require.NoError(t, repo.SaveDelivery(ctx, d))
		require.NotNil(t, d.NextRetryAt)
		require.True(t, d.NextRetryAt.Before(time.Now()), "delivery must be past-due")
		return d
	}

	now := time.Now()
	// Insert A (saved first → smallest updated_at) then B (saved later → larger
	// updated_at), but B is due earlier. Ordering by updated_at would yield
	// [A, B, ...]; ordering by effective due-time must yield [B, A, ...].
	a := mkFailed(`{"k":"a"}`, now.Add(-100*time.Second)) // due ≈ now-70s
	b := mkFailed(`{"k":"b"}`, now.Add(-300*time.Second)) // due ≈ now-270s (earliest)

	// C never attempted: next_retry_at is NULL → effective due = created_at ≈ now
	// (latest of the three).
	c, err := domain.NewDelivery(userID, ep.ID, domain.EventLeadCreated, []byte(`{"k":"c"}`))
	require.NoError(t, err)
	require.NoError(t, repo.EnqueueDelivery(ctx, c))

	due, err := repo.ClaimDueDeliveries(ctx, 10, 5)
	require.NoError(t, err)
	require.Len(t, due, 3)

	got := []uuid.UUID{due[0].ID, due[1].ID, due[2].ID}
	want := []uuid.UUID{b.ID, a.ID, c.ID}
	assert.Equal(t, want, got,
		"claim must return due deliveries earliest-due first (next_retry_at asc, NULL treated as created_at)")
}

// EnqueueDelivery must participate in an ambient transaction (transactional
// outbox, #199): when the caller wraps it in db.WithTx, the delivery row shares
// that transaction's fate — rolled back with the domain write, or committed with
// it. This is the core atomicity guarantee that closes the at-most-once gap
// (domain commit succeeds, separate enqueue fails → event lost).
func TestRepository_EnqueueDelivery_ParticipatesInAmbientTx(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := webhooks.NewRepository(pool, testutil.NewSecretCipher(t))
	tm := db.NewTxManager(pool)

	ep := mustEndpoint(t, userID, domain.EventLeadCreated)
	require.NoError(t, repo.CreateEndpoint(ctx, ep))

	countByID := func(id uuid.UUID) int {
		var n int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM webhook_deliveries WHERE id = $1`, id).Scan(&n))
		return n
	}

	// Rollback path: an enqueue inside a tx that fails must leave NO row.
	rolled, _ := domain.NewDelivery(userID, ep.ID, domain.EventLeadCreated, []byte(`{"k":"rollback"}`))
	boom := errors.New("boom")
	err := tm.WithTx(ctx, func(txCtx context.Context) error {
		if e := repo.EnqueueDelivery(txCtx, rolled); e != nil {
			return e
		}
		return boom // force rollback after a successful enqueue
	})
	require.ErrorIs(t, err, boom)
	assert.Equal(t, 0, countByID(rolled.ID),
		"enqueue in a rolled-back tx must not persist — the outbox row shares the domain tx")

	// Commit path: an enqueue inside a tx that succeeds must persist exactly one row.
	committed, _ := domain.NewDelivery(userID, ep.ID, domain.EventLeadCreated, []byte(`{"k":"commit"}`))
	require.NoError(t, tm.WithTx(ctx, func(txCtx context.Context) error {
		return repo.EnqueueDelivery(txCtx, committed)
	}))
	assert.Equal(t, 1, countByID(committed.ID),
		"enqueue in a committed tx must persist exactly one row")
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
