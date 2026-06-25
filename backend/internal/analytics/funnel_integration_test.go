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
			dto, err := repo.GetQualificationDistribution(context.Background(), userID, tt.step)
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

	dto, err := repo.GetQualificationDistribution(context.Background(), userID, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, dto.Total, "only this tenant's qualification is counted")
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

	dto, err := repo.GetSequenceConversion(context.Background(), userID)
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
