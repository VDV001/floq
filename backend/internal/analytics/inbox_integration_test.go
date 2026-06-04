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

// seedPendingReply inserts a pending_replies row. decidedAt is nil for an
// undecided row; otherwise it stamps decided_at so time-to-decide is
// decidedAt-createdAt.
func seedPendingReply(t *testing.T, pool *pgxpool.Pool, userID, leadID uuid.UUID, status string, createdAt time.Time, decidedAt *time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO pending_replies (id, user_id, lead_id, channel, kind, body, status, created_at, decided_at)
		 VALUES ($1, $2, $3, 'telegram', 'booking_link', 'hi there', $4, $5, $6)`,
		uuid.New(), userID, leadID, status, createdAt, decidedAt)
	require.NoError(t, err, "seed pending reply")
}

// allWindow is a [epoch, now+1h) window — the integration equivalent of
// PeriodAll, wide enough to include every freshly seeded row.
func allWindow() (time.Time, time.Time) {
	return time.Unix(0, 0).UTC(), time.Now().UTC().Add(time.Hour)
}

func TestRepository_GetInboxFlow_LeadsBreakdown(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	other := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)
	now := time.Now().UTC()

	seedHLLead(t, pool, userID, "telegram", "new", "a", now.Add(-time.Hour), now)
	seedHLLead(t, pool, userID, "telegram", "qualified", "b", now.Add(-time.Hour), now)
	seedHLLead(t, pool, userID, "email", "qualified", "c", now.Add(-time.Hour), now)
	seedHLLead(t, pool, userID, "email", "closed", "d", now.Add(-time.Hour), now)
	// Other tenant — must never appear.
	seedHLLead(t, pool, other, "telegram", "new", "x", now.Add(-time.Hour), now)
	// Old lead — outside a week window.
	seedHLLead(t, pool, userID, "telegram", "new", "old", now.Add(-60*24*time.Hour), now)

	from, to := allWindow()
	dto, err := repo.GetInboxFlow(context.Background(), userID, from, to)
	require.NoError(t, err)

	assert.Equal(t, 5, dto.Leads.Total, "all of this user's leads in the full window")
	assert.Equal(t, 3, dto.Leads.ByChannel["telegram"])
	assert.Equal(t, 2, dto.Leads.ByChannel["email"])
	assert.Equal(t, 2, dto.Leads.ByStatus["new"])
	assert.Equal(t, 2, dto.Leads.ByStatus["qualified"])
	assert.Equal(t, 1, dto.Leads.ByStatus["closed"])

	// Week window excludes the 60-day-old lead.
	weekFrom := now.Add(-7 * 24 * time.Hour)
	wk, err := repo.GetInboxFlow(context.Background(), userID, weekFrom, to)
	require.NoError(t, err)
	assert.Equal(t, 4, wk.Leads.Total, "old lead excluded by window")
}

func TestRepository_GetInboxFlow_QualificationHistogram(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)
	now := time.Now().UTC()

	for _, sc := range []int{10, 30, 90, 90} {
		l := seedHLLead(t, pool, userID, "telegram", "qualified", "q", now.Add(-time.Hour), now)
		seedHLQual(t, pool, l, sc, "r", now)
	}
	// A lead with no qualification must not affect the histogram or avg.
	seedHLLead(t, pool, userID, "telegram", "new", "noqual", now.Add(-time.Hour), now)

	from, to := allWindow()
	dto, err := repo.GetInboxFlow(context.Background(), userID, from, to)
	require.NoError(t, err)

	h := dto.Qualifications.ScoreHistogram
	require.Len(t, h, 5, "all five 0-100 bands present even when empty")
	assert.Equal(t, "0-20", h[0].Range)
	assert.Equal(t, 1, h[0].Count) // score 10
	assert.Equal(t, "21-40", h[1].Range)
	assert.Equal(t, 1, h[1].Count) // score 30
	assert.Equal(t, "41-60", h[2].Range)
	assert.Equal(t, 0, h[2].Count)
	assert.Equal(t, "61-80", h[3].Range)
	assert.Equal(t, 0, h[3].Count)
	assert.Equal(t, "81-100", h[4].Range)
	assert.Equal(t, 2, h[4].Count) // two score 90
	assert.InDelta(t, 55.0, dto.Qualifications.AvgScore, 0.001, "(10+30+90+90)/4")
}

func TestRepository_GetInboxFlow_QualificationAvgZeroWhenEmpty(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)

	from, to := allWindow()
	dto, err := repo.GetInboxFlow(context.Background(), userID, from, to)
	require.NoError(t, err)
	assert.Equal(t, 0.0, dto.Qualifications.AvgScore, "no quals → zero avg, not NULL scan error")
	require.Len(t, dto.Qualifications.ScoreHistogram, 5)
	for _, b := range dto.Qualifications.ScoreHistogram {
		assert.Equal(t, 0, b.Count)
	}
}

func TestRepository_GetInboxFlow_PendingReplies(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	other := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)
	now := time.Now().UTC()
	base := now.Add(-2 * time.Hour)
	lead := seedHLLead(t, pool, userID, "telegram", "qualified", "l", now.Add(-3*time.Hour), now)
	otherLead := seedHLLead(t, pool, other, "telegram", "qualified", "ol", now.Add(-3*time.Hour), now)

	dec := func(sec int) *time.Time { d := base.Add(time.Duration(sec) * time.Second); return &d }
	seedPendingReply(t, pool, userID, lead, "approved", base, dec(100))
	seedPendingReply(t, pool, userID, lead, "sent", base, dec(200))
	seedPendingReply(t, pool, userID, lead, "rejected", base, dec(300))
	seedPendingReply(t, pool, userID, lead, "pending", base, nil)
	// Other tenant — excluded.
	seedPendingReply(t, pool, other, otherLead, "approved", base, dec(999))

	from, to := allWindow()
	dto, err := repo.GetInboxFlow(context.Background(), userID, from, to)
	require.NoError(t, err)

	pr := dto.PendingReplies
	assert.Equal(t, 2, pr.Approved, "approved + sent both count as operator approval")
	assert.Equal(t, 1, pr.Rejected)
	assert.Equal(t, 1, pr.CurrentlyPending)
	// time-to-decide over decided rows {100,200,300}s.
	assert.Equal(t, 200, pr.P50TimeToDecideSeconds, "median")
	assert.Equal(t, 290, pr.P95TimeToDecideSeconds, "0.95 interpolated: 200 + 0.9*(300-200)")
}

func TestRepository_GetInboxFlow_PendingRepliesEmpty(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)

	from, to := allWindow()
	dto, err := repo.GetInboxFlow(context.Background(), userID, from, to)
	require.NoError(t, err)
	pr := dto.PendingReplies
	assert.Equal(t, 0, pr.Approved)
	assert.Equal(t, 0, pr.Rejected)
	assert.Equal(t, 0, pr.CurrentlyPending)
	assert.Equal(t, 0, pr.P50TimeToDecideSeconds, "no decided rows → zero percentile, not NULL scan error")
	assert.Equal(t, 0, pr.P95TimeToDecideSeconds)
}
