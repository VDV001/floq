//go:build integration

package leads_test

import (
	"context"
	"testing"
	"time"

	"github.com/daniil/floq/internal/leads"
	"github.com/daniil/floq/internal/leads/domain"
	"github.com/daniil/floq/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedLead(t *testing.T, repo *leads.Repository, userID uuid.UUID, status domain.LeadStatus) *domain.Lead {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	l := &domain.Lead{
		ID:          uuid.New(),
		UserID:      userID,
		Channel:     domain.ChannelEmail,
		ContactName: "Contact-" + uuid.New().String()[:8],
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	require.NoError(t, repo.CreateLead(context.Background(), l))
	return l
}

func TestListLeads_ExcludesArchived(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	active := seedLead(t, repo, userID, domain.StatusNew)
	archived := seedLead(t, repo, userID, domain.StatusNew)
	_, err := pool.Exec(ctx, `UPDATE leads SET archived_at = now() WHERE id = $1`, archived.ID)
	require.NoError(t, err)

	list, err := repo.ListLeads(ctx, userID)
	require.NoError(t, err)

	ids := make(map[uuid.UUID]bool, len(list))
	for _, l := range list {
		ids[l.ID] = true
	}
	assert.True(t, ids[active.ID], "active lead must appear in ListLeads")
	assert.False(t, ids[archived.ID], "archived lead must be excluded from ListLeads")
}

func TestListArchivedLeads_OnlyArchivedNewestFirst(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	other := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	active := seedLead(t, repo, userID, domain.StatusNew)
	older := seedLead(t, repo, userID, domain.StatusNew)
	newer := seedLead(t, repo, userID, domain.StatusNew)
	foreign := seedLead(t, repo, other, domain.StatusNew)

	olderAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newerAt := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `UPDATE leads SET archived_at = $1 WHERE id = $2`, olderAt, older.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE leads SET archived_at = $1 WHERE id = $2`, newerAt, newer.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE leads SET archived_at = now() WHERE id = $1`, foreign.ID)
	require.NoError(t, err)

	list, err := repo.ListArchivedLeads(ctx, userID)
	require.NoError(t, err)

	require.Len(t, list, 2, "only this user's archived leads")
	// Newest archived first.
	assert.Equal(t, newer.ID, list[0].ID, "most recently archived lead comes first")
	assert.Equal(t, older.ID, list[1].ID)
	require.NotNil(t, list[0].ArchivedAt, "archived_at is populated for the view")
	assert.Equal(t, newerAt, list[0].ArchivedAt.UTC())

	ids := map[uuid.UUID]bool{}
	for _, l := range list {
		ids[l.ID] = true
	}
	assert.False(t, ids[active.ID], "active lead excluded from archive view")
	assert.False(t, ids[foreign.ID], "another user's archived lead is not visible")
}

func TestListArchivedLeads_IncludesSourceName(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	categoryID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO source_categories (id, user_id, name, created_at) VALUES ($1, $2, 'Inbound', now())`,
		categoryID, userID)
	require.NoError(t, err)
	sourceID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO lead_sources (id, user_id, category_id, name, created_at) VALUES ($1, $2, $3, 'Website', now())`,
		sourceID, userID, categoryID)
	require.NoError(t, err)

	lead := seedLead(t, repo, userID, domain.StatusNew)
	_, err = pool.Exec(ctx, `UPDATE leads SET source_id = $1, archived_at = now() WHERE id = $2`, sourceID, lead.ID)
	require.NoError(t, err)

	list, err := repo.ListArchivedLeads(ctx, userID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Website", list[0].SourceName, "archive view joins source name like the inbox feed")
}

func TestStaleLeadsWithoutReminder_ExcludesArchived(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	// A stale lead: last message older than the staleness window.
	archived := seedLead(t, repo, userID, domain.StatusNew)
	_, err := pool.Exec(ctx,
		`INSERT INTO messages (id, lead_id, direction, body, sent_at) VALUES ($1, $2, 'inbound', 'hi', now() - interval '30 days')`,
		uuid.New(), archived.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE leads SET archived_at = now() WHERE id = $1`, archived.ID)
	require.NoError(t, err)

	stale, err := repo.StaleLeadsWithoutReminder(ctx, 7)
	require.NoError(t, err)
	for _, l := range stale {
		assert.NotEqual(t, archived.ID, l.ID, "archived lead must not be flagged stale")
	}
}

func TestCountLeads_ExcludeArchived(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	seedLead(t, repo, userID, domain.StatusNew)
	archived := seedLead(t, repo, userID, domain.StatusNew)
	_, err := pool.Exec(ctx, `UPDATE leads SET archived_at = now() WHERE id = $1`, archived.ID)
	require.NoError(t, err)

	total, err := repo.CountTotalLeads(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 1, total, "archived lead must not be counted in total usage")

	month, err := repo.CountMonthLeads(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 1, month, "archived lead must not be counted in month usage")
}

func TestSetLeadArchived_RejectsDoubleArchive(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	lead := seedLead(t, repo, userID, domain.StatusNew)
	now := time.Now().UTC()
	require.NoError(t, repo.SetLeadArchived(ctx, lead.ID, &now))

	// A second archive of the same lead must be rejected at the persistence
	// boundary (guards the concurrent read-check-write race, not just the
	// single-threaded domain check).
	err := repo.SetLeadArchived(ctx, lead.ID, &now)
	assert.ErrorIs(t, err, domain.ErrAlreadyArchived)
}

func TestSetLeadArchived_RejectsUnarchiveWhenNotArchived(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	lead := seedLead(t, repo, userID, domain.StatusNew)
	err := repo.SetLeadArchived(ctx, lead.ID, nil)
	assert.ErrorIs(t, err, domain.ErrNotArchived)
}

func TestExportCSV_IncludesArchived(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	uc := leads.NewUseCase(repo, nil, nil)
	ctx := context.Background()

	active := seedLead(t, repo, userID, domain.StatusNew)
	archived := seedLead(t, repo, userID, domain.StatusNew)
	_, err := pool.Exec(ctx, `UPDATE leads SET archived_at = now() WHERE id = $1`, archived.ID)
	require.NoError(t, err)

	data, err := uc.ExportCSV(ctx, userID)
	require.NoError(t, err)
	csv := string(data)

	assert.Contains(t, csv, active.ContactName, "active lead in export")
	assert.Contains(t, csv, archived.ContactName, "archived lead must be included in the CSV backup")
	assert.Contains(t, csv, "archived_at", "export carries an archived_at column header")
}

func TestExportImport_RoundTripsArchiveTimestamp(t *testing.T) {
	pool := testutil.TestDB(t)
	userA := testutil.SeedUser(t, pool)
	userB := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	uc := leads.NewUseCase(repo, nil, nil)
	ctx := context.Background()

	lead := seedLead(t, repo, userA, domain.StatusNew)
	archivedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	_, err := pool.Exec(ctx, `UPDATE leads SET archived_at = $1, email_address = $2 WHERE id = $3`, archivedAt, "rt@example.com", lead.ID)
	require.NoError(t, err)

	data, err := uc.ExportCSV(ctx, userA)
	require.NoError(t, err)

	n, err := uc.ImportCSV(ctx, userB, data)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, 1)

	all, err := repo.ListAllLeads(ctx, userB)
	require.NoError(t, err)
	var imported *domain.LeadWithSource
	for i := range all {
		if all[i].EmailAddress != nil && *all[i].EmailAddress == "rt@example.com" {
			imported = &all[i]
		}
	}
	require.NotNil(t, imported, "archived lead restored for user B")
	require.NotNil(t, imported.ArchivedAt, "restored lead stays archived")
	assert.Equal(t, archivedAt, imported.ArchivedAt.UTC(), "exact archive timestamp round-trips through export→import")
}

func TestSetLeadArchived_RoundTrip(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	lead := seedLead(t, repo, userID, domain.StatusQualified)

	now := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, repo.SetLeadArchived(ctx, lead.ID, &now))

	got, err := repo.GetLead(ctx, lead.ID)
	require.NoError(t, err)
	require.NotNil(t, got.ArchivedAt, "ArchivedAt must be persisted and scanned")
	assert.WithinDuration(t, now, *got.ArchivedAt, time.Second)
	assert.Equal(t, domain.StatusQualified, got.Status, "archive must not change status")

	// Unarchive path: nil clears it.
	require.NoError(t, repo.SetLeadArchived(ctx, lead.ID, nil))
	got, err = repo.GetLead(ctx, lead.ID)
	require.NoError(t, err)
	assert.Nil(t, got.ArchivedAt, "ArchivedAt must be cleared on unarchive")
}
