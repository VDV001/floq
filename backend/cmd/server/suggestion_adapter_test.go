//go:build integration

package main

import (
	"context"
	"testing"
	"time"

	"github.com/daniil/floq/internal/db"
	"github.com/daniil/floq/internal/leads"
	leadsdomain "github.com/daniil/floq/internal/leads/domain"
	"github.com/daniil/floq/internal/prospects"
	prospectsdomain "github.com/daniil/floq/internal/prospects/domain"
	"github.com/daniil/floq/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Exercises the ownership-enforcement contract of the ProspectSuggestionFinder
// adapter: every mutation and read MUST refuse to cross tenants, and the
// "not yours" case is indistinguishable from "doesn't exist" (same sentinel).

func seedLead(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, name, company string) uuid.UUID {
	t.Helper()
	now := time.Now().UTC()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO leads (id, user_id, channel, contact_name, company, first_message, status, created_at, updated_at)
		 VALUES ($1, $2, 'email', $3, $4, 'hi', 'new', $5, $5)`,
		id, userID, name, company, now)
	require.NoError(t, err)
	return id
}

func seedProspect(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, name, company string) uuid.UUID {
	t.Helper()
	p, err := prospectsdomain.NewProspect(userID, name, company, "", "", "manual")
	require.NoError(t, err)
	repo := prospects.NewRepository(pool)
	require.NoError(t, repo.CreateProspect(context.Background(), p))
	return p.ID
}

func TestAdapter_FindForLead_RejectsForeignLead(t *testing.T) {
	pool := testutil.TestDB(t)
	alice := testutil.SeedUser(t, pool)
	bob := testutil.SeedUser(t, pool)
	adapter := newProspectSuggestionFinderAdapter(db.NewTxManager(pool), leads.NewRepository(pool), prospects.NewRepository(pool))

	aliceLead := seedLead(t, pool, alice, "X", "Y")

	// Bob asks for suggestions on Alice's lead — must be rejected with
	// ErrLeadNotFound, NOT an empty result (which would leak existence).
	_, err := adapter.FindForLead(context.Background(), bob, aliceLead)
	assert.ErrorIs(t, err, leadsdomain.ErrLeadNotFound)
}

func TestAdapter_FindForLead_UnknownLeadReturnsSameSentinel(t *testing.T) {
	pool := testutil.TestDB(t)
	alice := testutil.SeedUser(t, pool)
	adapter := newProspectSuggestionFinderAdapter(db.NewTxManager(pool), leads.NewRepository(pool), prospects.NewRepository(pool))

	_, err := adapter.FindForLead(context.Background(), alice, uuid.New())
	assert.ErrorIs(t, err, leadsdomain.ErrLeadNotFound)
}

func TestAdapter_LinkProspect_RejectsForeignLead(t *testing.T) {
	pool := testutil.TestDB(t)
	alice := testutil.SeedUser(t, pool)
	bob := testutil.SeedUser(t, pool)
	adapter := newProspectSuggestionFinderAdapter(db.NewTxManager(pool), leads.NewRepository(pool), prospects.NewRepository(pool))

	aliceLead := seedLead(t, pool, alice, "X", "Y")
	bobProspect := seedProspect(t, pool, bob, "Bob", "BobCo")

	// Bob tries to link his prospect to Alice's lead — must fail; nothing
	// is written (prospect stays new, dismissals table untouched).
	err := adapter.LinkProspect(context.Background(), bob, aliceLead, bobProspect)
	assert.ErrorIs(t, err, leadsdomain.ErrLeadNotFound)

	p, err := prospects.NewRepository(pool).GetProspect(context.Background(), bobProspect)
	require.NoError(t, err)
	assert.Equal(t, prospectsdomain.ProspectStatusNew, p.Status)
}

func TestAdapter_LinkProspect_RejectsForeignProspect(t *testing.T) {
	pool := testutil.TestDB(t)
	alice := testutil.SeedUser(t, pool)
	bob := testutil.SeedUser(t, pool)
	adapter := newProspectSuggestionFinderAdapter(db.NewTxManager(pool), leads.NewRepository(pool), prospects.NewRepository(pool))

	aliceLead := seedLead(t, pool, alice, "X", "Y")
	bobProspect := seedProspect(t, pool, bob, "Bob", "BobCo")

	// Alice tries to link Bob's prospect to her own lead — must fail with
	// ErrProspectNotFound.
	err := adapter.LinkProspect(context.Background(), alice, aliceLead, bobProspect)
	assert.ErrorIs(t, err, leadsdomain.ErrProspectNotFound)
}

func TestAdapter_LinkProspect_AtomicSuccess(t *testing.T) {
	pool := testutil.TestDB(t)
	user := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	adapter := newProspectSuggestionFinderAdapter(db.NewTxManager(pool), leads.NewRepository(pool), repo)

	leadID := seedLead(t, pool, user, "Даниил", "Floq")

	// Prospect with a source_id that the lead should inherit.
	categoryID := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO source_categories (id, user_id, name) VALUES ($1, $2, 'Test')`, categoryID, user)
	require.NoError(t, err)
	sourceID := uuid.New()
	_, err = pool.Exec(context.Background(),
		`INSERT INTO lead_sources (id, user_id, category_id, name) VALUES ($1, $2, $3, 'Test')`,
		sourceID, user, categoryID)
	require.NoError(t, err)

	prospect, err := prospectsdomain.NewProspect(user, "Даниил", "Floq", "", "", "manual")
	require.NoError(t, err)
	prospect.SourceID = &sourceID
	require.NoError(t, repo.CreateProspect(context.Background(), prospect))

	require.NoError(t, adapter.LinkProspect(context.Background(), user, leadID, prospect.ID))

	// Prospect now converted + converted_lead_id set.
	got, err := repo.GetProspect(context.Background(), prospect.ID)
	require.NoError(t, err)
	assert.Equal(t, prospectsdomain.ProspectStatusConverted, got.Status)
	require.NotNil(t, got.ConvertedLeadID)
	assert.Equal(t, leadID, *got.ConvertedLeadID)

	// Lead inherited the source_id.
	var gotSourceID *uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT source_id FROM leads WHERE id = $1`, leadID).Scan(&gotSourceID))
	require.NotNil(t, gotSourceID)
	assert.Equal(t, sourceID, *gotSourceID)
}

func TestAdapter_DismissSuggestion_RejectsForeignLead(t *testing.T) {
	pool := testutil.TestDB(t)
	alice := testutil.SeedUser(t, pool)
	bob := testutil.SeedUser(t, pool)
	adapter := newProspectSuggestionFinderAdapter(db.NewTxManager(pool), leads.NewRepository(pool), prospects.NewRepository(pool))

	aliceLead := seedLead(t, pool, alice, "X", "Y")
	bobProspect := seedProspect(t, pool, bob, "Bob", "BobCo")

	err := adapter.DismissSuggestion(context.Background(), bob, aliceLead, bobProspect)
	assert.ErrorIs(t, err, leadsdomain.ErrLeadNotFound)

	// Dismissals table stayed empty.
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM prospect_suggestion_dismissals WHERE lead_id = $1`, aliceLead).Scan(&n))
	assert.Equal(t, 0, n)
}

func TestAdapter_DismissSuggestion_RejectsForeignProspect(t *testing.T) {
	pool := testutil.TestDB(t)
	alice := testutil.SeedUser(t, pool)
	bob := testutil.SeedUser(t, pool)
	adapter := newProspectSuggestionFinderAdapter(db.NewTxManager(pool), leads.NewRepository(pool), prospects.NewRepository(pool))

	aliceLead := seedLead(t, pool, alice, "X", "Y")
	bobProspect := seedProspect(t, pool, bob, "Bob", "BobCo")

	err := adapter.DismissSuggestion(context.Background(), alice, aliceLead, bobProspect)
	assert.ErrorIs(t, err, leadsdomain.ErrProspectNotFound)
}
