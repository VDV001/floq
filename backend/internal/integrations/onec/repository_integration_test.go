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

	inserted, err := repo.InsertSyncRecord(ctx, rec1)
	require.NoError(t, err)
	require.True(t, inserted, "first insert must persist")

	// Same (user, external_id, external_type) → dedup, no second row.
	rec2, err := domain.NewSyncRecord(user, newEvent(t, "ОП-0001"), domain.DirectionInbound)
	require.NoError(t, err)
	inserted, err = repo.InsertSyncRecord(ctx, rec2)
	require.NoError(t, err)
	require.False(t, inserted, "replayed event must be deduped")

	// Different external_id → new row.
	rec3, err := domain.NewSyncRecord(user, newEvent(t, "ОП-0002"), domain.DirectionInbound)
	require.NoError(t, err)
	inserted, err = repo.InsertSyncRecord(ctx, rec3)
	require.NoError(t, err)
	require.True(t, inserted, "distinct event must persist")
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
	insertedA, err := repo.InsertSyncRecord(ctx, recA)
	require.NoError(t, err)
	require.True(t, insertedA)

	recB, err := domain.NewSyncRecord(userB, newEvent(t, "ОП-SHARED"), domain.DirectionInbound)
	require.NoError(t, err)
	insertedB, err := repo.InsertSyncRecord(ctx, recB)
	require.NoError(t, err)
	require.True(t, insertedB, "same external id under different user must not collide")
}
