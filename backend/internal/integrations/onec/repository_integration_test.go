//go:build integration

package onec_test

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/integrations/onec"
	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/daniil/floq/internal/testutil"
	"github.com/stretchr/testify/require"
)

func newEvent(t *testing.T, extID string) *domain.ExternalEvent {
	t.Helper()
	ev, err := domain.NewExternalEvent(extID, "Документ.ОплатаПокупателя", domain.EventKindPayment, []byte(`{"sum":1000}`))
	require.NoError(t, err)
	return ev
}

func TestRepository_InsertSyncRecord_Dedup(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := onec.NewRepository(pool)
	ctx := context.Background()
	user := testutil.SeedUser(t, pool)

	rec1, err := domain.NewSyncRecord(user, newEvent(t, "ОП-0001"), domain.DirectionInbound)
	require.NoError(t, err)

	out, err := repo.InsertSyncRecord(ctx, rec1)
	require.NoError(t, err)
	require.True(t, out.Inserted, "first insert must persist")

	// Same (user, external_id, external_type) → dedup, no second row.
	rec2, err := domain.NewSyncRecord(user, newEvent(t, "ОП-0001"), domain.DirectionInbound)
	require.NoError(t, err)
	out, err = repo.InsertSyncRecord(ctx, rec2)
	require.NoError(t, err)
	require.False(t, out.Inserted, "replayed event must be deduped")

	// Different external_id → new row.
	rec3, err := domain.NewSyncRecord(user, newEvent(t, "ОП-0002"), domain.DirectionInbound)
	require.NoError(t, err)
	out, err = repo.InsertSyncRecord(ctx, rec3)
	require.NoError(t, err)
	require.True(t, out.Inserted, "distinct event must persist")
}

func TestRepository_InsertSyncRecord_PerUserScope(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := onec.NewRepository(pool)
	ctx := context.Background()
	userA := testutil.SeedUser(t, pool)
	userB := testutil.SeedUser(t, pool)

	// Same external id for two different users must both persist —
	// dedup is per-user, not global.
	recA, err := domain.NewSyncRecord(userA, newEvent(t, "ОП-SHARED"), domain.DirectionInbound)
	require.NoError(t, err)
	outA, err := repo.InsertSyncRecord(ctx, recA)
	require.NoError(t, err)
	require.True(t, outA.Inserted)

	recB, err := domain.NewSyncRecord(userB, newEvent(t, "ОП-SHARED"), domain.DirectionInbound)
	require.NoError(t, err)
	outB, err := repo.InsertSyncRecord(ctx, recB)
	require.NoError(t, err)
	require.True(t, outB.Inserted, "same external id under different user must not collide")
}

func TestRepository_InsertSyncRecord_PayloadDrift(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := onec.NewRepository(pool)
	ctx := context.Background()
	user := testutil.SeedUser(t, pool)

	mk := func(payload string) *domain.SyncRecord {
		ev, err := domain.NewExternalEvent("ОП-DRIFT", "Документ.Оплата", domain.EventKindPayment, []byte(payload))
		require.NoError(t, err)
		rec, err := domain.NewSyncRecord(user, ev, domain.DirectionInbound)
		require.NoError(t, err)
		return rec
	}

	out, err := repo.InsertSyncRecord(ctx, mk(`{"sum":1000}`))
	require.NoError(t, err)
	require.True(t, out.Inserted)
	require.False(t, out.PayloadDrifted)

	// Same dedup key, identical payload → deduped, no drift.
	out, err = repo.InsertSyncRecord(ctx, mk(`{"sum":1000}`))
	require.NoError(t, err)
	require.False(t, out.Inserted)
	require.False(t, out.PayloadDrifted)

	// Same dedup key, CHANGED payload → deduped but drift detected.
	out, err = repo.InsertSyncRecord(ctx, mk(`{"sum":9999}`))
	require.NoError(t, err)
	require.False(t, out.Inserted)
	require.True(t, out.PayloadDrifted, "changed payload under same external id must be flagged as drift")
}

func TestRepository_UserIDByWebhookSecret(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := onec.NewRepository(pool)
	ctx := context.Background()
	user := testutil.SeedUser(t, pool)

	_, err := pool.Exec(ctx,
		`INSERT INTO onec_credentials (user_id, webhook_secret, is_active) VALUES ($1, $2, TRUE)`,
		user, "s3cr3t")
	require.NoError(t, err)

	t.Run("active secret resolves", func(t *testing.T) {
		got, found, err := repo.UserIDByWebhookSecret(ctx, "s3cr3t")
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, user, got)
	})

	t.Run("empty secret never matches", func(t *testing.T) {
		_, found, err := repo.UserIDByWebhookSecret(ctx, "")
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("unknown secret not found", func(t *testing.T) {
		_, found, err := repo.UserIDByWebhookSecret(ctx, "wrong")
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("inactive credentials excluded", func(t *testing.T) {
		_, err := pool.Exec(ctx, `UPDATE onec_credentials SET is_active = FALSE WHERE user_id = $1`, user)
		require.NoError(t, err)
		_, found, err := repo.UserIDByWebhookSecret(ctx, "s3cr3t")
		require.NoError(t, err)
		require.False(t, found, "inactive credentials must not authenticate")
	})
}

func TestRepository_InsertSyncRecord_AlreadyProcessedAndMark(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := onec.NewRepository(pool)
	ctx := context.Background()
	user := testutil.SeedUser(t, pool)

	rec, err := domain.NewSyncRecord(user, newEvent(t, "ОП-9001"), domain.DirectionInbound)
	require.NoError(t, err)

	out, err := repo.InsertSyncRecord(ctx, rec)
	require.NoError(t, err)
	require.True(t, out.Inserted)
	require.False(t, out.AlreadyProcessed, "a fresh record starts unprocessed")

	// Replay before processing → dedup, still not processed (reconciliation must re-apply).
	out, err = repo.InsertSyncRecord(ctx, rec)
	require.NoError(t, err)
	require.False(t, out.Inserted)
	require.False(t, out.AlreadyProcessed, "recorded-but-unapplied must read as not-processed")

	// Apply succeeded → mark processed.
	require.NoError(t, repo.MarkProcessed(ctx, rec))

	// Replay after processing → dedup AND already-processed (no re-apply).
	out, err = repo.InsertSyncRecord(ctx, rec)
	require.NoError(t, err)
	require.False(t, out.Inserted)
	require.True(t, out.AlreadyProcessed, "a processed record must read as already-processed")
}
