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

func archiveLead(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `UPDATE leads SET archived_at = now() WHERE id = $1`, id)
	require.NoError(t, err, "archive lead")
}

func TestRepository_GetInboxFlow_ExcludesArchived(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)
	now := time.Now().UTC()

	seedHLLead(t, pool, userID, "telegram", "new", "active", now.Add(-time.Hour), now)
	archived := seedHLLead(t, pool, userID, "telegram", "new", "archived", now.Add(-time.Hour), now)
	archiveLead(t, pool, archived)

	from, to := allWindow()
	dto, err := repo.GetInboxFlow(context.Background(), userID, from, to)
	require.NoError(t, err)

	assert.Equal(t, 1, dto.Leads.Total, "archived lead must not be counted in inbox breakdown")
	assert.Equal(t, 1, dto.Leads.ByStatus["new"])
}

func TestRepository_GetInboxFlow_QualDistributionExcludesArchived(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)
	now := time.Now().UTC()

	active := seedHLLead(t, pool, userID, "telegram", "qualified", "active", now.Add(-time.Hour), now)
	seedHLQual(t, pool, active, 40, "ok", now)
	archived := seedHLLead(t, pool, userID, "telegram", "qualified", "archived", now.Add(-time.Hour), now)
	seedHLQual(t, pool, archived, 90, "great", now)
	archiveLead(t, pool, archived)

	from, to := allWindow()
	dto, err := repo.GetInboxFlow(context.Background(), userID, from, to)
	require.NoError(t, err)

	// Only the active lead's score (40) should feed the histogram; the
	// archived 90 must be excluded → average is 40, not 65.
	assert.InDelta(t, 40.0, dto.Qualifications.AvgScore, 0.001, "archived qualification must not feed the histogram")
}

func TestRepository_GetHotLeads_ExcludesArchived(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)
	now := time.Now().UTC()

	active := seedHLLead(t, pool, userID, "telegram", "qualified", "active", now.Add(-time.Hour), now)
	seedHLQual(t, pool, active, 90, "great", now)
	archived := seedHLLead(t, pool, userID, "telegram", "qualified", "archived", now.Add(-time.Hour), now)
	seedHLQual(t, pool, archived, 95, "best", now)
	archiveLead(t, pool, archived)

	dto, err := repo.GetHotLeads(context.Background(), userID, anyFilter())
	require.NoError(t, err)

	require.Len(t, dto.Leads, 1, "archived lead must not appear in hot leads")
	assert.Equal(t, active, dto.Leads[0].ID)
	assert.Equal(t, 1, dto.TotalMatching)
}

func TestRepository_GetCostRatios_ExcludesArchived(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := analytics.NewRepository(pool)
	now := time.Now().UTC()

	active := seedLead(t, pool, userID, "qualified", now.Add(-time.Hour))
	seedQualification(t, pool, active, 90)
	archived := seedLead(t, pool, userID, "qualified", now.Add(-time.Hour))
	seedQualification(t, pool, archived, 95)
	archiveLead(t, pool, archived)

	got, err := repo.GetCostRatios(context.Background(), userID, now.Add(-7*24*time.Hour), now.Add(time.Hour))
	require.NoError(t, err)

	assert.Equal(t, 1, got.LeadsCount, "archived lead must not be counted")
	assert.Equal(t, 1, got.QualifiedLeadsCount, "archived qualified lead must not be counted")
}
