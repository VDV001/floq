//go:build integration

package analytics_test

import (
	"context"
	"testing"
	"time"

	"github.com/daniil/floq/internal/analytics"
	"github.com/daniil/floq/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedSequence inserts a sequence row tied to userID and returns its id.
func seedSequence(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO sequences (id, user_id, name, is_active, created_at)
		 VALUES ($1, $2, $3, true, NOW())`, id, userID, name)
	require.NoError(t, err, "seed sequence")
	return id
}

// seedProspect inserts a prospect tied to userID with the given status.
// email uses a uuid suffix so concurrent test runs don't collide on
// any unique constraints.
func seedProspect(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	email := id.String() + "@test.local"
	_, err := pool.Exec(context.Background(),
		`INSERT INTO prospects (id, user_id, name, email, status, verify_status, created_at, updated_at)
		 VALUES ($1, $2, 'Test Prospect', $3, $4, 'valid', NOW(), NOW())`,
		id, userID, email, status)
	require.NoError(t, err, "seed prospect")
	return id
}

// seedOutbound inserts an outbound_messages row. opened/replied are
// optional pointers — nil means the column stays NULL.
func seedOutbound(t *testing.T, pool *pgxpool.Pool, prospectID, sequenceID uuid.UUID, status string, openedAt, repliedAt *time.Time, createdAt time.Time) {
	t.Helper()
	id := uuid.New()
	var sentAt *time.Time
	if status == "sent" || status == "bounced" {
		t := createdAt
		sentAt = &t
	}
	_, err := pool.Exec(context.Background(),
		`INSERT INTO outbound_messages (id, prospect_id, sequence_id, step_order, channel, body, status, scheduled_at, sent_at, opened_at, replied_at, created_at)
		 VALUES ($1, $2, $3, 1, 'email', 'hi', $4::outbound_status, $5, $6, $7, $8, $9)`,
		id, prospectID, sequenceID, status, createdAt, sentAt, openedAt, repliedAt, createdAt)
	require.NoError(t, err, "seed outbound")
}

func TestRepository_GetSequenceStats_EmptyWhenNoSequences(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)

	got, err := repo.GetSequenceStats(context.Background(), userID, analytics.PeriodAll)
	require.NoError(t, err)
	assert.Empty(t, got, "user with no sequences returns empty slice, not nil and not error")
}

func TestRepository_GetSequenceStats_AggregatesCounts(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	seqID := seedSequence(t, pool, userID, "Cold Outreach IT")

	pA := seedProspect(t, pool, userID, "in_sequence")
	pB := seedProspect(t, pool, userID, "in_sequence")
	pConv := seedProspect(t, pool, userID, "converted")
	pBounced := seedProspect(t, pool, userID, "in_sequence")

	now := time.Now().UTC()
	openedAt := now.Add(-1 * time.Hour)
	repliedAt := now.Add(-30 * time.Minute)

	// pA: sent + opened + replied. One row.
	seedOutbound(t, pool, pA, seqID, "sent", &openedAt, &repliedAt, now)
	// pB: sent, opened but not replied.
	seedOutbound(t, pool, pB, seqID, "sent", &openedAt, nil, now)
	// pConv: sent (no open, no reply) but prospect later converted.
	seedOutbound(t, pool, pConv, seqID, "sent", nil, nil, now)
	// pBounced: bounced. Counts in sent, NOT in delivered.
	seedOutbound(t, pool, pBounced, seqID, "bounced", nil, nil, now)

	got, err := analytics.NewRepository(pool).GetSequenceStats(context.Background(), userID, analytics.PeriodAll)
	require.NoError(t, err)
	require.Len(t, got, 1)

	row := got[0]
	assert.Equal(t, seqID, row.ID)
	assert.Equal(t, "Cold Outreach IT", row.Name)
	assert.EqualValues(t, 4, row.Sent, "sent counts both 'sent' and 'bounced'")
	assert.EqualValues(t, 3, row.Delivered, "delivered counts only 'sent'")
	assert.EqualValues(t, 2, row.Opened, "opened counts rows where opened_at is not null")
	assert.EqualValues(t, 1, row.Replied, "replied counts rows where replied_at is not null")
	assert.EqualValues(t, 1, row.Converted, "converted counts DISTINCT prospects with status='converted'")
}

func TestRepository_GetSequenceStats_ScopedByUser(t *testing.T) {
	pool := testutil.TestDB(t)
	userA := testutil.SeedUser(t, pool)
	userB := testutil.SeedUser(t, pool)
	seqA := seedSequence(t, pool, userA, "A-only")
	seqB := seedSequence(t, pool, userB, "B-only")

	pA := seedProspect(t, pool, userA, "in_sequence")
	pB := seedProspect(t, pool, userB, "in_sequence")
	now := time.Now().UTC()
	seedOutbound(t, pool, pA, seqA, "sent", nil, nil, now)
	seedOutbound(t, pool, pB, seqB, "sent", nil, nil, now)

	repo := analytics.NewRepository(pool)

	gotA, err := repo.GetSequenceStats(context.Background(), userA, analytics.PeriodAll)
	require.NoError(t, err)
	require.Len(t, gotA, 1)
	assert.Equal(t, seqA, gotA[0].ID, "user A must only see their own sequence")

	gotB, err := repo.GetSequenceStats(context.Background(), userB, analytics.PeriodAll)
	require.NoError(t, err)
	require.Len(t, gotB, 1)
	assert.Equal(t, seqB, gotB[0].ID, "user B must only see their own sequence")
}

func TestRepository_GetSequenceStats_PeriodFilter(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	seqID := seedSequence(t, pool, userID, "Window test")
	prospectID := seedProspect(t, pool, userID, "in_sequence")

	now := time.Now().UTC()
	// One message inside the week window.
	seedOutbound(t, pool, prospectID, seqID, "sent", nil, nil, now.Add(-2*24*time.Hour))
	// One message outside the week window (10 days old) but inside month.
	seedOutbound(t, pool, prospectID, seqID, "sent", nil, nil, now.Add(-10*24*time.Hour))
	// One message outside both windows (40 days old).
	seedOutbound(t, pool, prospectID, seqID, "sent", nil, nil, now.Add(-40*24*time.Hour))

	repo := analytics.NewRepository(pool)

	week, err := repo.GetSequenceStats(context.Background(), userID, analytics.PeriodWeek)
	require.NoError(t, err)
	require.Len(t, week, 1)
	assert.EqualValues(t, 1, week[0].Sent, "week period must include only outbound from last 7 days")

	month, err := repo.GetSequenceStats(context.Background(), userID, analytics.PeriodMonth)
	require.NoError(t, err)
	require.Len(t, month, 1)
	assert.EqualValues(t, 2, month[0].Sent, "month period must include last 30 days")

	all, err := repo.GetSequenceStats(context.Background(), userID, analytics.PeriodAll)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.EqualValues(t, 3, all[0].Sent, "all period must include every row")
}

func TestRepository_GetSequenceStats_ConvertedDistinctPerProspect(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	seqID := seedSequence(t, pool, userID, "Distinct test")
	converted := seedProspect(t, pool, userID, "converted")

	now := time.Now().UTC()
	// Same converted prospect received 3 outbound messages — converted
	// must count as 1, not 3 (DISTINCT prospect_id).
	seedOutbound(t, pool, converted, seqID, "sent", nil, nil, now)
	seedOutbound(t, pool, converted, seqID, "sent", nil, nil, now)
	seedOutbound(t, pool, converted, seqID, "sent", nil, nil, now)

	got, err := analytics.NewRepository(pool).GetSequenceStats(context.Background(), userID, analytics.PeriodAll)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.EqualValues(t, 1, got[0].Converted, "converted must be DISTINCT prospect_id, not message count")
}
