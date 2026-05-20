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

// seedAuditCost inserts an audit_log row charging costMicro to userID
// at createdAt. Status defaults to 'success' — the cost-ratios view
// counts every recorded call, success or not.
func seedAuditCost(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, costMicro int64, createdAt time.Time) {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO audit_log (id, user_id, request_type, provider, model,
			input_tokens, output_tokens, total_tokens,
			cost_usd_micro, latency_ms, status, created_at)
		 VALUES ($1, $2, 'qualification', 'test', 'test-model', 100, 50, 150, $3, 100, 'success', $4)`,
		id, userID, costMicro, createdAt)
	require.NoError(t, err, "seed audit cost")
}

// seedLead inserts a leads row at createdAt. status default 'new'.
func seedLead(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, status string, createdAt time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO leads (id, user_id, channel, contact_name, first_message, status, created_at, updated_at)
		 VALUES ($1, $2, 'telegram', 'Test', 'hi', $3::lead_status, $4, $4)`,
		id, userID, status, createdAt)
	require.NoError(t, err, "seed lead")
	return id
}

// seedQualification attaches a qualification row with the given score
// to leadID. qualifications.lead_id is UNIQUE — one row per lead via
// UPSERT in production. generated_at defaults to NOW().
func seedQualification(t *testing.T, pool *pgxpool.Pool, leadID uuid.UUID, score int) {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO qualifications (id, lead_id, identified_need, estimated_budget, deadline, score, score_reason, recommended_action, provider_used, generated_at)
		 VALUES ($1, $2, '', '', '', $3, '', '', 'test', NOW())
		 ON CONFLICT (lead_id) DO UPDATE SET score = EXCLUDED.score, generated_at = EXCLUDED.generated_at`,
		id, leadID, score)
	require.NoError(t, err, "seed qualification")
}

// updateProspect mutates a seeded prospect into converted state at
// updatedAt — used to position conversions inside / outside the window.
func updateProspect(t *testing.T, pool *pgxpool.Pool, prospectID uuid.UUID, leadID uuid.UUID, updatedAt time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`UPDATE prospects SET status = 'converted', converted_lead_id = $2, updated_at = $3 WHERE id = $1`,
		prospectID, leadID, updatedAt)
	require.NoError(t, err, "update prospect to converted")
}

// seedSentOutbound inserts an outbound_messages row already in 'sent'
// state with sent_at = createdAt.
func seedSentOutbound(t *testing.T, pool *pgxpool.Pool, prospectID, sequenceID uuid.UUID, sentAt time.Time) {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO outbound_messages (id, prospect_id, sequence_id, step_order, channel, body, status, scheduled_at, sent_at, created_at)
		 VALUES ($1, $2, $3, 1, 'email', 'hi', 'sent'::outbound_status, $4, $4, $4)`,
		id, prospectID, sequenceID, sentAt)
	require.NoError(t, err, "seed sent outbound")
}

func TestRepository_GetCostRatios_EmptyWhenNoData(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)

	now := time.Now().UTC()
	got, err := repo.GetCostRatios(context.Background(), userID, now.Add(-7*24*time.Hour), now)
	require.NoError(t, err)
	require.NotNil(t, got, "empty period must return zero-valued DTO, not nil")
	assert.Zero(t, got.TotalCostUSDMicro)
	assert.Zero(t, got.TotalCalls)
	assert.Zero(t, got.LeadsCount)
	assert.Zero(t, got.QualifiedLeadsCount)
	assert.Zero(t, got.ConvertedCount)
	assert.Zero(t, got.DraftsSentCount)
	assert.Zero(t, got.CostPerLeadUSDMicro, "zero leads must yield zero ratio (no div-by-zero)")
}

func TestRepository_GetCostRatios_AggregatesCounts(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)

	now := time.Now().UTC()
	from := now.Add(-7 * 24 * time.Hour)

	// 3 audit calls: 1, 2, 3 USD micro = 6 total
	seedAuditCost(t, pool, userID, 1_000_000, now.Add(-1*time.Hour))
	seedAuditCost(t, pool, userID, 2_000_000, now.Add(-2*time.Hour))
	seedAuditCost(t, pool, userID, 3_000_000, now.Add(-3*time.Hour))

	// 4 leads: 2 qualified (score >= 80), 2 not
	l1 := seedLead(t, pool, userID, "new", now.Add(-1*time.Hour))
	l2 := seedLead(t, pool, userID, "qualified", now.Add(-2*time.Hour))
	l3 := seedLead(t, pool, userID, "qualified", now.Add(-3*time.Hour))
	l4 := seedLead(t, pool, userID, "new", now.Add(-4*time.Hour))
	seedQualification(t, pool, l1, 50) // below threshold
	seedQualification(t, pool, l2, 85) // qualified
	seedQualification(t, pool, l3, 90) // qualified
	seedQualification(t, pool, l4, 70) // below

	// 1 converted prospect in window (updated_at inside)
	prospectID := seedProspect(t, pool, userID, "in_sequence")
	updateProspect(t, pool, prospectID, l2, now.Add(-30*time.Minute))

	// 5 sent outbound messages in window. Offset by -1m so the latest
	// row sits strictly before `to` (the half-open window's right
	// boundary excludes exactly-equal rows).
	seqID := seedSequence(t, pool, userID, "S1")
	for i := 0; i < 5; i++ {
		seedSentOutbound(t, pool, prospectID, seqID, now.Add(-1*time.Minute).Add(time.Duration(-i)*time.Hour))
	}

	got, err := repo.GetCostRatios(context.Background(), userID, from, now)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.EqualValues(t, 6_000_000, got.TotalCostUSDMicro)
	assert.Equal(t, 3, got.TotalCalls)
	assert.Equal(t, 4, got.LeadsCount, "4 leads created in window")
	assert.Equal(t, 2, got.QualifiedLeadsCount, "2 leads with max qualification score >= 80")
	assert.Equal(t, 1, got.ConvertedCount, "1 prospect updated to converted inside window")
	assert.Equal(t, 5, got.DraftsSentCount, "5 outbound messages with status='sent' inside window")

	assert.EqualValues(t, 1_500_000, got.CostPerLeadUSDMicro, "6_000_000 / 4 = 1_500_000")
	assert.EqualValues(t, 3_000_000, got.CostPerQualifiedUSDMicro, "6_000_000 / 2")
	assert.EqualValues(t, 6_000_000, got.CostPerConvertedUSDMicro, "6_000_000 / 1")
	assert.EqualValues(t, 1_200_000, got.CostPerDraftSentUSDMicro, "6_000_000 / 5")
}

func TestRepository_GetCostRatios_ScopedByUser(t *testing.T) {
	pool := testutil.TestDB(t)
	userA := testutil.SeedUser(t, pool)
	userB := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)

	now := time.Now().UTC()
	seedAuditCost(t, pool, userA, 5_000_000, now.Add(-1*time.Hour))
	seedAuditCost(t, pool, userB, 99_000_000, now.Add(-1*time.Hour))
	seedLead(t, pool, userA, "new", now.Add(-1*time.Hour))
	seedLead(t, pool, userB, "new", now.Add(-1*time.Hour))

	got, err := repo.GetCostRatios(context.Background(), userA, now.Add(-24*time.Hour), now)
	require.NoError(t, err)
	assert.EqualValues(t, 5_000_000, got.TotalCostUSDMicro, "must not include user B")
	assert.Equal(t, 1, got.LeadsCount, "must not include user B leads")
}

func TestRepository_GetCostRatios_WindowFilter(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)

	now := time.Now().UTC()
	from := now.Add(-7 * 24 * time.Hour)

	// inside window
	seedAuditCost(t, pool, userID, 1_000_000, now.Add(-1*time.Hour))
	seedLead(t, pool, userID, "new", now.Add(-2*time.Hour))
	// outside window (10 days old)
	seedAuditCost(t, pool, userID, 999_000_000, now.Add(-10*24*time.Hour))
	seedLead(t, pool, userID, "new", now.Add(-10*24*time.Hour))

	got, err := repo.GetCostRatios(context.Background(), userID, from, now)
	require.NoError(t, err)
	assert.EqualValues(t, 1_000_000, got.TotalCostUSDMicro)
	assert.Equal(t, 1, got.LeadsCount)
}

func TestRepository_GetCostRatios_QualifiedReQualification(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)

	now := time.Now().UTC()
	// qualifications.lead_id is UNIQUE — the production AI re-qualifier
	// UPSERTs over the existing row. A lead first scored below
	// threshold and later re-scored above must count as qualified now.
	leadID := seedLead(t, pool, userID, "qualified", now.Add(-1*time.Hour))
	seedQualification(t, pool, leadID, 40) // initial low score
	seedQualification(t, pool, leadID, 85) // re-qualification UPSERT overwrites

	got, err := repo.GetCostRatios(context.Background(), userID, now.Add(-7*24*time.Hour), now)
	require.NoError(t, err)
	assert.Equal(t, 1, got.QualifiedLeadsCount, "lead with latest score >= 80 counts as qualified after UPSERT")
}
