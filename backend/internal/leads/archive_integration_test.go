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
