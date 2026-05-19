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
