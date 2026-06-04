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

func seedHLLead(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, channel, status, name string, createdAt, updatedAt time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO leads (id, user_id, channel, contact_name, first_message, status, created_at, updated_at)
		 VALUES ($1, $2, $3::lead_channel, $4, 'hi', $5::lead_status, $6, $7)`,
		id, userID, channel, name, status, createdAt, updatedAt)
	require.NoError(t, err, "seed lead")
	return id
}

func seedHLQual(t *testing.T, pool *pgxpool.Pool, leadID uuid.UUID, score int, reason string, genAt time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO qualifications (id, lead_id, score, score_reason, generated_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		uuid.New(), leadID, score, reason, genAt)
	require.NoError(t, err, "seed qualification")
}

func anyFilter() analytics.HotLeadsFilter {
	return analytics.HotLeadsFilter{Period: analytics.PeriodAll, Status: analytics.FilterAny, Channel: analytics.FilterAny, Limit: 20}
}

func TestRepository_GetHotLeads_SortsByScoreThenActivityNullsLast(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)
	now := time.Now().UTC()

	hi := seedHLLead(t, pool, userID, "telegram", "qualified", "High", now.Add(-3*time.Hour), now.Add(-1*time.Hour))
	seedHLQual(t, pool, hi, 90, "strong fit", now)
	mid := seedHLLead(t, pool, userID, "email", "qualified", "Mid", now.Add(-2*time.Hour), now.Add(-30*time.Minute))
	seedHLQual(t, pool, mid, 50, "maybe", now)
	noqual := seedHLLead(t, pool, userID, "telegram", "new", "Unqualified", now.Add(-1*time.Hour), now)

	dto, err := repo.GetHotLeads(context.Background(), userID, anyFilter())
	require.NoError(t, err)
	require.Len(t, dto.Leads, 3)

	assert.Equal(t, hi, dto.Leads[0].ID, "highest score first")
	require.NotNil(t, dto.Leads[0].Score)
	assert.Equal(t, 90, *dto.Leads[0].Score)
	assert.Equal(t, "strong fit", dto.Leads[0].ScoreReason)
	require.NotNil(t, dto.Leads[0].QualifiedAt)

	assert.Equal(t, mid, dto.Leads[1].ID)
	assert.Equal(t, noqual, dto.Leads[2].ID, "unqualified (NULL score) sorts last")
	assert.Nil(t, dto.Leads[2].Score, "no qualification → nil score")
	assert.Nil(t, dto.Leads[2].QualifiedAt)
	assert.Equal(t, 3, dto.TotalMatching)
}

func TestRepository_GetHotLeads_StatusAndChannelFilters(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)
	now := time.Now().UTC()

	seedHLLead(t, pool, userID, "telegram", "qualified", "Q-tg", now, now)
	seedHLLead(t, pool, userID, "email", "new", "N-em", now, now)
	seedHLLead(t, pool, userID, "telegram", "closed", "C-tg", now, now)

	// status=any excludes terminal 'closed' by default.
	all, err := repo.GetHotLeads(context.Background(), userID, anyFilter())
	require.NoError(t, err)
	assert.Equal(t, 2, all.TotalMatching, "default any excludes closed")
	for _, l := range all.Leads {
		assert.NotEqual(t, "closed", l.Status)
	}

	// Explicit status=closed returns only the closed lead.
	f := anyFilter()
	f.Status = "closed"
	closed, err := repo.GetHotLeads(context.Background(), userID, f)
	require.NoError(t, err)
	require.Len(t, closed.Leads, 1)
	assert.Equal(t, "closed", closed.Leads[0].Status)

	// channel=email returns only email leads (and still excludes closed).
	f = anyFilter()
	f.Channel = "email"
	em, err := repo.GetHotLeads(context.Background(), userID, f)
	require.NoError(t, err)
	require.Len(t, em.Leads, 1)
	assert.Equal(t, "email", em.Leads[0].Channel)
}

func TestRepository_GetHotLeads_TenantScopedAndLimited(t *testing.T) {
	pool := testutil.TestDB(t)
	userA := testutil.SeedUser(t, pool)
	userB := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)
	now := time.Now().UTC()

	for i := 0; i < 3; i++ {
		l := seedHLLead(t, pool, userA, "telegram", "qualified", "A", now, now.Add(-time.Duration(i)*time.Hour))
		seedHLQual(t, pool, l, 90-i, "r", now)
	}
	lb := seedHLLead(t, pool, userB, "telegram", "qualified", "B", now, now)
	seedHLQual(t, pool, lb, 99, "r", now)

	f := anyFilter()
	f.Limit = 2
	dto, err := repo.GetHotLeads(context.Background(), userA, f)
	require.NoError(t, err)
	assert.Len(t, dto.Leads, 2, "limit applied to page")
	assert.Equal(t, 3, dto.TotalMatching, "total counts all matching pre-limit, userA only")
	for _, l := range dto.Leads {
		assert.NotEqual(t, lb, l.ID, "userB lead must never appear")
	}
}

func TestRepository_GetHotLeads_PeriodFiltersByCreatedAt(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)
	now := time.Now().UTC()

	recent := seedHLLead(t, pool, userID, "telegram", "qualified", "recent", now.Add(-2*24*time.Hour), now)
	seedHLQual(t, pool, recent, 80, "r", now)
	seedHLLead(t, pool, userID, "telegram", "qualified", "old", now.Add(-60*24*time.Hour), now)

	f := anyFilter()
	f.Period = analytics.PeriodWeek
	dto, err := repo.GetHotLeads(context.Background(), userID, f)
	require.NoError(t, err)
	require.Len(t, dto.Leads, 1, "only leads created within the week")
	assert.Equal(t, recent, dto.Leads[0].ID)
}
