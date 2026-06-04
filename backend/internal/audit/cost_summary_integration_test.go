//go:build integration

package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/daniil/floq/internal/audit"
	"github.com/daniil/floq/internal/audit/domain"
	"github.com/daniil/floq/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedAuditEntry inserts a minimal audit_log row tied to userID with
// the given request_type / model / cost / token counts. created_at
// defaults to NOW(); pass an explicit time to control window-filter
// tests.
func seedAuditEntry(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, requestType, model string, costMicro int64, inTokens, outTokens int, createdAt time.Time) {
	t.Helper()
	id := uuid.New()
	totalTokens := inTokens + outTokens
	_, err := pool.Exec(context.Background(),
		`INSERT INTO audit_log (id, user_id, request_type, provider, model,
			input_tokens, output_tokens, total_tokens,
			cost_usd_micro, latency_ms, status, created_at)
		 VALUES ($1, $2, $3, 'test-provider', $4, $5, $6, $7, $8, 100, 'success', $9)`,
		id, userID, requestType, model, inTokens, outTokens, totalTokens, costMicro, createdAt)
	require.NoError(t, err, "seed audit entry")
}

// seedDailyRollup inserts one pre-aggregated audit_log_daily bucket —
// the shape the retention cron leaves behind after purging old per-call
// rows. Used to assert the cost summary stitches recent detail together
// with aggregated history.
func seedDailyRollup(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, day time.Time, provider, model, reqType string, calls, costMicro, inTok, outTok int64) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO audit_log_daily
			(day, user_id, provider, model, request_type,
			 total_calls, total_cost_usd_micro, total_input_tokens, total_output_tokens)
		 VALUES ($1::date, $2, $3, $4, $5, $6, $7, $8, $9)`,
		day, userID, provider, model, reqType, calls, costMicro, inTok, outTok)
	require.NoError(t, err, "seed daily rollup")
}

func TestRepository_CostSummary_IncludesAggregatedHistory(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := audit.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	// One recent detailed row still in audit_log.
	seedAuditEntry(t, pool, userID, "qualification", "m1", 1_000_000, 100, 50, now)
	// Aggregated history in the daily rollup: same request_type+model so
	// the breakdowns must MERGE detail and rollup, not list them twice.
	oldDay := now.AddDate(0, 0, -40)
	seedDailyRollup(t, pool, userID, oldDay, "test-provider", "m1", "qualification", 5, 5_000_000, 500, 250)
	// A different model, only in history — must appear in by_model.
	seedDailyRollup(t, pool, userID, oldDay, "test-provider", "m2", "draft_reply", 2, 2_000_000, 200, 100)

	from := now.AddDate(0, 0, -60)
	to := now.Add(time.Hour)
	got, err := repo.CostSummary(ctx, userID, from, to)
	require.NoError(t, err)
	require.NotNil(t, got)

	// Totals span detail + history: 1 + 5 + 2 = 8 calls, 8 USD micro.
	assert.Equal(t, 8, got.TotalCalls)
	assert.EqualValues(t, 8_000_000, got.TotalUSDMicro)

	// by_request_type: qualification merges recent (1c) + history (5c) = 6c.
	byType := map[string]domain.RequestTypeBreakdown{}
	for _, b := range got.ByRequestType {
		byType[b.RequestType] = b
	}
	require.Contains(t, byType, "qualification")
	assert.Equal(t, 6, byType["qualification"].Calls, "recent detail + aggregated history merge")
	assert.EqualValues(t, 6_000_000, byType["qualification"].USDMicro)
	assert.EqualValues(t, 600, byType["qualification"].InputTokens)
	assert.EqualValues(t, 300, byType["qualification"].OutputTokens)
	require.Contains(t, byType, "draft_reply")
	assert.Equal(t, 2, byType["draft_reply"].Calls)

	// by_model: m1 merges detail+history (6c), m2 from history only (2c).
	byModel := map[string]domain.ModelBreakdown{}
	for _, b := range got.ByModel {
		byModel[b.Model] = b
	}
	assert.Equal(t, 6, byModel["m1"].Calls)
	assert.EqualValues(t, 6_000_000, byModel["m1"].USDMicro)
	assert.Equal(t, 2, byModel["m2"].Calls)
}

// TestRepository_CostSummary_SplitDayNoLossNoDoubleCount pins the
// tightest invariant of the union: a SINGLE calendar day whose rows are
// split between audit_log (afternoon, not yet purged) and
// audit_log_daily (morning, already rolled up by the cron) must sum to
// the day's true total — neither dropping the detailed tail nor
// double-counting the rolled-up head. The two sides hold different
// physical rows, so the outer SUM is exact.
func TestRepository_CostSummary_SplitDayNoLossNoDoubleCount(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := audit.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	// A day near the retention boundary, split across both tables.
	splitDay := now.AddDate(0, 0, -30)
	splitDayAfternoon := time.Date(splitDay.Year(), splitDay.Month(), splitDay.Day(), 18, 0, 0, 0, time.UTC)

	// Morning of splitDay: already purged → lives only in the rollup.
	seedDailyRollup(t, pool, userID, splitDay, "test-provider", "m1", "qualification", 3, 3_000_000, 300, 150)
	// Afternoon of the SAME day: still detailed in audit_log.
	seedAuditEntry(t, pool, userID, "qualification", "m1", 1_000_000, 100, 50, splitDayAfternoon)

	from := now.AddDate(0, 0, -60)
	to := now.Add(time.Hour)
	got, err := repo.CostSummary(ctx, userID, from, to)
	require.NoError(t, err)

	assert.Equal(t, 4, got.TotalCalls, "morning rollup (3) + afternoon detail (1), no loss/no double-count")
	assert.EqualValues(t, 4_000_000, got.TotalUSDMicro)
	require.Len(t, got.ByRequestType, 1)
	assert.Equal(t, "qualification", got.ByRequestType[0].RequestType)
	assert.Equal(t, 4, got.ByRequestType[0].Calls)
	assert.EqualValues(t, 400, got.ByRequestType[0].InputTokens)
	assert.EqualValues(t, 200, got.ByRequestType[0].OutputTokens)
}

func TestRepository_CostSummary_DailyRollupScopedByUserAndRange(t *testing.T) {
	pool := testutil.TestDB(t)
	userA := testutil.SeedUser(t, pool)
	userB := testutil.SeedUser(t, pool)
	repo := audit.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	// userB's history must never leak into userA's summary.
	seedDailyRollup(t, pool, userB, now.AddDate(0, 0, -40), "test-provider", "m1", "qualification", 99, 99_000_000, 9, 9)
	// userA history inside and outside the queried range.
	seedDailyRollup(t, pool, userA, now.AddDate(0, 0, -40), "test-provider", "m1", "qualification", 5, 5_000_000, 50, 25)
	seedDailyRollup(t, pool, userA, now.AddDate(0, 0, -400), "test-provider", "m1", "qualification", 7, 7_000_000, 70, 35)

	from := now.AddDate(0, 0, -60)
	to := now.Add(time.Hour)
	got, err := repo.CostSummary(ctx, userA, from, to)
	require.NoError(t, err)
	assert.Equal(t, 5, got.TotalCalls, "only userA's in-range history counts")
	assert.EqualValues(t, 5_000_000, got.TotalUSDMicro, "cross-tenant + out-of-range history excluded")
}

func TestRepository_CostSummary_AggregatesByRequestTypeAndModel(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := audit.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	// Three qualification calls on haiku (1c, 2c, 3c micro = 6c total)
	seedAuditEntry(t, pool, userID, "qualification", "claude-haiku-4-5", 1_000_000, 100, 50, now)
	seedAuditEntry(t, pool, userID, "qualification", "claude-haiku-4-5", 2_000_000, 200, 100, now)
	seedAuditEntry(t, pool, userID, "qualification", "claude-haiku-4-5", 3_000_000, 300, 150, now)
	// Two cold-message calls on opus (5c, 7c micro = 12c total)
	seedAuditEntry(t, pool, userID, "cold_message", "claude-opus-4-7", 5_000_000, 400, 200, now)
	seedAuditEntry(t, pool, userID, "cold_message", "claude-opus-4-7", 7_000_000, 500, 250, now)
	// Total: 5 calls, 18 USD micro

	from := now.AddDate(0, 0, -1)
	to := now.AddDate(0, 0, 1)
	got, err := repo.CostSummary(ctx, userID, from, to)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, 5, got.TotalCalls, "5 entries seeded total")
	assert.EqualValues(t, 18_000_000, got.TotalUSDMicro, "sum of all costs")

	// by_request_type ordered by spend desc — cold_message (12) before qualification (6)
	require.Len(t, got.ByRequestType, 2)
	assert.Equal(t, "cold_message", got.ByRequestType[0].RequestType, "highest-spend type ranks first")
	assert.Equal(t, 2, got.ByRequestType[0].Calls)
	assert.EqualValues(t, 12_000_000, got.ByRequestType[0].USDMicro)
	assert.EqualValues(t, 900, got.ByRequestType[0].InputTokens)
	assert.EqualValues(t, 450, got.ByRequestType[0].OutputTokens)
	assert.Equal(t, "qualification", got.ByRequestType[1].RequestType)
	assert.Equal(t, 3, got.ByRequestType[1].Calls)
	assert.EqualValues(t, 6_000_000, got.ByRequestType[1].USDMicro)

	// by_model ordered by spend desc — opus (12) before haiku (6)
	require.Len(t, got.ByModel, 2)
	assert.Equal(t, "claude-opus-4-7", got.ByModel[0].Model)
	assert.EqualValues(t, 12_000_000, got.ByModel[0].USDMicro)
	assert.Equal(t, "claude-haiku-4-5", got.ByModel[1].Model)
	assert.EqualValues(t, 6_000_000, got.ByModel[1].USDMicro)
}

func TestRepository_CostSummary_PeriodFiltersOutOfRange(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := audit.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	inWindow := now.Add(-12 * time.Hour)
	outOfWindow := now.AddDate(0, 0, -10)

	seedAuditEntry(t, pool, userID, "qualification", "m1", 1_000_000, 10, 5, inWindow)
	seedAuditEntry(t, pool, userID, "qualification", "m1", 9_000_000, 90, 45, outOfWindow)

	// Window: last day only.
	from := now.AddDate(0, 0, -1)
	to := now.Add(time.Hour) // include now
	got, err := repo.CostSummary(ctx, userID, from, to)
	require.NoError(t, err)
	assert.Equal(t, 1, got.TotalCalls, "out-of-window row must be excluded")
	assert.EqualValues(t, 1_000_000, got.TotalUSDMicro)
}

func TestRepository_CostSummary_ScopedByUser(t *testing.T) {
	pool := testutil.TestDB(t)
	userA := testutil.SeedUser(t, pool)
	userB := testutil.SeedUser(t, pool)
	repo := audit.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	seedAuditEntry(t, pool, userA, "qualification", "m1", 5_000_000, 100, 50, now)
	seedAuditEntry(t, pool, userB, "qualification", "m1", 99_000_000, 9999, 4999, now)

	from := now.AddDate(0, 0, -1)
	to := now.Add(time.Hour)
	got, err := repo.CostSummary(ctx, userA, from, to)
	require.NoError(t, err)
	assert.Equal(t, 1, got.TotalCalls, "user A sees only their own rows")
	assert.EqualValues(t, 5_000_000, got.TotalUSDMicro, "cross-tenant cost MUST NOT leak")
}

func TestRepository_CostSummary_EmptyRangeReturnsZeroBreakdowns(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := audit.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	got, err := repo.CostSummary(ctx, userID, now.AddDate(0, 0, -1), now)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 0, got.TotalCalls)
	assert.EqualValues(t, 0, got.TotalUSDMicro)
	// Empty slices, not nil — JSON edge depends on it.
	assert.NotNil(t, got.ByRequestType)
	assert.Len(t, got.ByRequestType, 0)
	assert.NotNil(t, got.ByModel)
	assert.Len(t, got.ByModel, 0)
}
