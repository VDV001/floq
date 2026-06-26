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

// refreshFunnelMatviews materialises the funnel views after seeding so the
// read-path sees the freshly inserted rows. Plain (non-concurrent) REFRESH
// is fine in tests; the production cron uses CONCURRENTLY.
func refreshFunnelMatviews(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	for _, mv := range []string{
		"mv_analytics_qualification_distribution",
		"mv_analytics_sequence_step_conversion",
	} {
		_, err := pool.Exec(context.Background(), "REFRESH MATERIALIZED VIEW "+mv)
		require.NoError(t, err, "refresh %s", mv)
	}
}

// seedStepOutbound inserts a sent outbound for a prospect at a specific
// step_order (the shared seedOutbound hardcodes step 1). repliedAt marks a
// reply to that step.
func seedStepOutbound(t *testing.T, pool *pgxpool.Pool, prospectID, sequenceID uuid.UUID, stepOrder int, repliedAt *time.Time) {
	t.Helper()
	now := time.Now().UTC()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO outbound_messages (id, prospect_id, sequence_id, step_order, channel, body, status, scheduled_at, sent_at, replied_at, created_at)
		 VALUES ($1, $2, $3, $4, 'email', 'hi', 'sent'::outbound_status, $5, $5, $6, $5)`,
		uuid.New(), prospectID, sequenceID, stepOrder, now, repliedAt)
	require.NoError(t, err, "seed step outbound")
}

// seedQualificationAt inserts a qualification with an explicit generated_at so
// a test can place it inside or outside a period window.
func seedQualificationAt(t *testing.T, pool *pgxpool.Pool, leadID uuid.UUID, score int, generatedAt time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO qualifications (id, lead_id, identified_need, estimated_budget, deadline, score, score_reason, recommended_action, provider_used, generated_at)
		 VALUES ($1, $2, '', '', '', $3, '', '', 'test', $4)
		 ON CONFLICT (lead_id) DO UPDATE SET score = EXCLUDED.score, generated_at = EXCLUDED.generated_at`,
		uuid.New(), leadID, score, generatedAt)
	require.NoError(t, err, "seed qualification at")
}

// seedStepOutboundAt inserts a sent outbound at an explicit sent_at so a test
// can place a step entry inside or outside a period window.
func seedStepOutboundAt(t *testing.T, pool *pgxpool.Pool, prospectID, sequenceID uuid.UUID, stepOrder int, sentAt time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO outbound_messages (id, prospect_id, sequence_id, step_order, channel, body, status, scheduled_at, sent_at, created_at)
		 VALUES ($1, $2, $3, $4, 'email', 'hi', 'sent'::outbound_status, $5, $5, $5)`,
		uuid.New(), prospectID, sequenceID, stepOrder, sentAt)
	require.NoError(t, err, "seed step outbound at")
}

// Period windows: a week/month window must exclude rows older than its cutoff;
// PeriodAll counts everything. Exercises both funnel read-paths.
func TestRepository_Funnel_PeriodWindows(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)
	now := time.Now().UTC()
	recent := now.Add(-2 * 24 * time.Hour) // inside week + month
	old := now.Add(-40 * 24 * time.Hour)   // outside week + month

	lRecent := seedLead(t, pool, userID, "qualified", recent)
	seedQualificationAt(t, pool, lRecent, 45, recent)
	lOld := seedLead(t, pool, userID, "qualified", old)
	seedQualificationAt(t, pool, lOld, 45, old)

	seqID := seedSequence(t, pool, userID, "Win")
	pRecent := seedProspect(t, pool, userID, "new")
	pOld := seedProspect(t, pool, userID, "new")
	seedStepOutboundAt(t, pool, pRecent, seqID, 1, recent)
	seedStepOutboundAt(t, pool, pOld, seqID, 1, old)
	refreshFunnelMatviews(t, pool)

	tests := []struct {
		name          string
		period        analytics.Period
		wantQualTotal int
		wantEntered   int
	}{
		{"all", analytics.PeriodAll, 2, 2},
		{"week", analytics.PeriodWeek, 1, 1},
		{"month", analytics.PeriodMonth, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qd, err := repo.GetQualificationDistribution(context.Background(), userID, 10, tt.period)
			require.NoError(t, err)
			assert.Equal(t, tt.wantQualTotal, qd.Total, "qualification total for %s window", tt.name)

			conv, err := repo.GetSequenceConversion(context.Background(), userID, tt.period)
			require.NoError(t, err)
			entered := 0
			for _, s := range conv.Steps {
				if s.SequenceID == seqID && s.StepOrder == 1 {
					entered = s.Entered
				}
			}
			assert.Equal(t, tt.wantEntered, entered, "entered count for %s window", tt.name)
		})
	}
}

func TestRepository_GetQualificationDistribution_FoldsToConfiguredStep(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)
	now := time.Now().UTC()

	// One qualification in each of the lowest five width-10 bins.
	for _, score := range []int{5, 15, 25, 35, 45} {
		leadID := seedLead(t, pool, userID, "qualified", now)
		seedQualification(t, pool, leadID, score)
	}
	refreshFunnelMatviews(t, pool)

	tests := []struct {
		name string
		step int
		want map[int]int // bucket_lo -> count (only the non-zero ones asserted)
	}{
		{"step10", 10, map[int]int{0: 1, 10: 1, 20: 1, 30: 1, 40: 1}},
		{"step20", 20, map[int]int{0: 2, 20: 2, 40: 1}},
		{"step50", 50, map[int]int{0: 5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dto, err := repo.GetQualificationDistribution(context.Background(), userID, tt.step, analytics.PeriodAll)
			require.NoError(t, err)
			require.Equal(t, tt.step, dto.Step)
			assert.Equal(t, 5, dto.Total, "all five qualifications counted")

			got := map[int]int{}
			for _, b := range dto.Buckets {
				got[b.Lo] = b.Count
			}
			for lo, want := range tt.want {
				assert.Equalf(t, want, got[lo], "bucket starting at %d", lo)
			}

			// The top band must be inclusive of the maximum score (100), not
			// the dead "step-1" upper bound.
			top := dto.Buckets[len(dto.Buckets)-1]
			assert.Equal(t, 100, top.Hi, "top bucket upper bound is 100")
			assert.Contains(t, top.Label, "–100", "top bucket label closes at 100")
		})
	}
}

func TestRepository_GetQualificationDistribution_TenantScoped(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	other := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)
	now := time.Now().UTC()

	mine := seedLead(t, pool, userID, "qualified", now)
	seedQualification(t, pool, mine, 42)
	theirs := seedLead(t, pool, other, "qualified", now)
	seedQualification(t, pool, theirs, 88)
	refreshFunnelMatviews(t, pool)

	dto, err := repo.GetQualificationDistribution(context.Background(), userID, 10, analytics.PeriodAll)
	require.NoError(t, err)
	assert.Equal(t, 1, dto.Total, "only this tenant's qualification is counted")
}

func TestRepository_RefreshMatviews_Concurrently(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)
	now := time.Now().UTC()

	leadID := seedLead(t, pool, userID, "qualified", now)
	seedQualification(t, pool, leadID, 73)

	// REFRESH ... CONCURRENTLY needs the UNIQUE index on each view and a
	// view already populated once (created WITH DATA). This is exactly the
	// path the background cron drives, so a green run proves it works.
	require.NoError(t, repo.RefreshMatviews(context.Background()))

	dto, err := repo.GetQualificationDistribution(context.Background(), userID, 10, analytics.PeriodAll)
	require.NoError(t, err)
	assert.Equal(t, 1, dto.Total, "concurrent refresh picked up the seeded qualification")
}

func TestRepository_GetSequenceConversion(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)
	now := time.Now().UTC()

	seqID := seedSequence(t, pool, userID, "Warm intro")

	// 3 prospects receive step 1. 2 of them reply. 1 advances to step 2.
	p1 := seedProspect(t, pool, userID, "new")
	p2 := seedProspect(t, pool, userID, "new")
	p3 := seedProspect(t, pool, userID, "new")
	seedStepOutbound(t, pool, p1, seqID, 1, &now)
	seedStepOutbound(t, pool, p2, seqID, 1, &now)
	seedStepOutbound(t, pool, p3, seqID, 1, nil)
	seedStepOutbound(t, pool, p1, seqID, 2, nil) // p1 advanced to step 2
	refreshFunnelMatviews(t, pool)

	dto, err := repo.GetSequenceConversion(context.Background(), userID, analytics.PeriodAll)
	require.NoError(t, err)
	require.NotEmpty(t, dto.Steps)

	var step1 *analytics.SequenceStepConversionDTO
	for i := range dto.Steps {
		if dto.Steps[i].SequenceID == seqID && dto.Steps[i].StepOrder == 1 {
			step1 = &dto.Steps[i]
		}
	}
	require.NotNil(t, step1, "step 1 row present")
	assert.Equal(t, "Warm intro", step1.SequenceName)
	assert.Equal(t, 3, step1.Entered)
	assert.Equal(t, 2, step1.Replied)
	assert.Equal(t, 1, step1.Advanced)
	assert.InDelta(t, 2.0/3.0, step1.ReplyRate, 0.001)
	assert.InDelta(t, 1.0/3.0, step1.AdvanceRate, 0.001)
}
