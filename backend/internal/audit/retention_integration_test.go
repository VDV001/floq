//go:build integration

package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/daniil/floq/internal/audit"
	"github.com/daniil/floq/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dailyRow mirrors one audit_log_daily row for assertions.
type dailyRow struct {
	day        time.Time
	calls      int64
	costMicro  int64
	inTokens   int64
	outTokens  int64
}

func readDailyRows(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) []dailyRow {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT day, total_calls, total_cost_usd_micro, total_input_tokens, total_output_tokens
		 FROM audit_log_daily WHERE user_id = $1 ORDER BY day`, userID)
	require.NoError(t, err)
	defer rows.Close()
	var out []dailyRow
	for rows.Next() {
		var r dailyRow
		require.NoError(t, rows.Scan(&r.day, &r.calls, &r.costMicro, &r.inTokens, &r.outTokens))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

func countAuditLog(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM audit_log WHERE user_id = $1`, userID).Scan(&n))
	return n
}

func TestRepository_AggregateAndPurge_RollsUpOldRowsAndDeletesThem(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := audit.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	threshold := now.AddDate(0, 0, -30)
	oldDay := now.AddDate(0, 0, -40)

	// Two old rows on the same (day, provider, model, request_type) must
	// collapse into a single daily bucket with summed counts.
	seedAuditEntry(t, pool, userID, "qualification", "m1", 1_000_000, 100, 50, oldDay)
	seedAuditEntry(t, pool, userID, "qualification", "m1", 2_000_000, 200, 100, oldDay)
	// A recent row (newer than the threshold) must be left untouched.
	seedAuditEntry(t, pool, userID, "qualification", "m1", 9_000_000, 900, 450, now)

	purged, err := repo.AggregateAndPurgeOlderThan(ctx, threshold)
	require.NoError(t, err)
	assert.Equal(t, 2, purged, "two rows older than threshold purged")

	// Only the recent row survives in audit_log.
	assert.Equal(t, 1, countAuditLog(t, pool, userID))

	daily := readDailyRows(t, pool, userID)
	require.Len(t, daily, 1, "two old rows of same key collapse to one daily bucket")
	assert.EqualValues(t, 2, daily[0].calls)
	assert.EqualValues(t, 3_000_000, daily[0].costMicro)
	assert.EqualValues(t, 300, daily[0].inTokens)
	assert.EqualValues(t, 150, daily[0].outTokens)
	assert.Equal(t, oldDay.Format("2006-01-02"), daily[0].day.Format("2006-01-02"), "bucketed by calendar day of created_at")
}

func TestRepository_AggregateAndPurge_SkipsRecentRows(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := audit.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	threshold := now.AddDate(0, 0, -30)

	// All rows are recent — nothing to purge.
	seedAuditEntry(t, pool, userID, "qualification", "m1", 1_000_000, 10, 5, now)
	seedAuditEntry(t, pool, userID, "draft_reply", "m2", 2_000_000, 20, 10, now.AddDate(0, 0, -5))

	purged, err := repo.AggregateAndPurgeOlderThan(ctx, threshold)
	require.NoError(t, err)
	assert.Equal(t, 0, purged, "no row older than threshold")
	assert.Equal(t, 2, countAuditLog(t, pool, userID), "recent rows untouched")
	assert.Empty(t, readDailyRows(t, pool, userID), "nothing rolled up")
}

func TestRepository_AggregateAndPurge_IsIdempotent(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := audit.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	threshold := now.AddDate(0, 0, -30)
	oldDay := now.AddDate(0, 0, -40)

	seedAuditEntry(t, pool, userID, "qualification", "m1", 4_000_000, 400, 200, oldDay)

	purged1, err := repo.AggregateAndPurgeOlderThan(ctx, threshold)
	require.NoError(t, err)
	assert.Equal(t, 1, purged1)

	// Second run with the same threshold: the rows are already gone, so
	// nothing is purged and the daily bucket must NOT be double-counted.
	purged2, err := repo.AggregateAndPurgeOlderThan(ctx, threshold)
	require.NoError(t, err)
	assert.Equal(t, 0, purged2, "idempotent: second pass purges nothing")

	daily := readDailyRows(t, pool, userID)
	require.Len(t, daily, 1)
	assert.EqualValues(t, 1, daily[0].calls, "bucket not inflated by re-run")
	assert.EqualValues(t, 4_000_000, daily[0].costMicro)
}

func TestRepository_AggregateAndPurge_AccumulatesAcrossRuns(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := audit.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	oldDay := now.AddDate(0, 0, -40)

	// First batch aged past the threshold and rolled up.
	seedAuditEntry(t, pool, userID, "qualification", "m1", 1_000_000, 100, 50, oldDay)
	purged1, err := repo.AggregateAndPurgeOlderThan(ctx, now.AddDate(0, 0, -30))
	require.NoError(t, err)
	require.Equal(t, 1, purged1)

	// A second row on the SAME calendar day surfaces later (e.g. a
	// delayed flush) and ages past a later threshold — it must ADD to
	// the existing daily bucket, not create a duplicate PK row.
	seedAuditEntry(t, pool, userID, "qualification", "m1", 3_000_000, 300, 150, oldDay)
	purged2, err := repo.AggregateAndPurgeOlderThan(ctx, now.AddDate(0, 0, -30))
	require.NoError(t, err)
	require.Equal(t, 1, purged2)

	daily := readDailyRows(t, pool, userID)
	require.Len(t, daily, 1, "same key accumulates into one bucket via ON CONFLICT")
	assert.EqualValues(t, 2, daily[0].calls)
	assert.EqualValues(t, 4_000_000, daily[0].costMicro)
	assert.EqualValues(t, 400, daily[0].inTokens)
	assert.EqualValues(t, 200, daily[0].outTokens)
}
