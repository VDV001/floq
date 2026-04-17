//go:build integration

package prospects_test

import (
	"context"
	"testing"
	"time"

	"github.com/daniil/floq/internal/prospects"
	"github.com/daniil/floq/internal/prospects/domain"
	"github.com/daniil/floq/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createLead inserts a lead row for FK references used by suggestion tests.
func createLead(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, name, company, email string) uuid.UUID {
	t.Helper()
	now := time.Now().UTC()
	leadID := uuid.New()
	channel := "email"
	var emailPtr *string
	if email != "" {
		emailPtr = &email
	}
	_, err := pool.Exec(context.Background(),
		`INSERT INTO leads (id, user_id, channel, contact_name, company, first_message, status, email_address, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, 'hi', 'new', $6, $7, $7)`,
		leadID, userID, channel, name, company, emailPtr, now)
	require.NoError(t, err)
	return leadID
}

func seedProspectWith(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, name, company, email, tgUsername string) *domain.Prospect {
	t.Helper()
	p, _ := domain.NewProspect(userID, name, company, "", email, "manual")
	p.TelegramUsername = tgUsername
	repo := prospects.NewRepository(pool)
	require.NoError(t, repo.CreateProspect(context.Background(), p))
	return p
}

func TestFindSuggestionsForLead_AllTiers(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	// Lead: Даниил @ floq.dev, company=Floq, email=dan@floq.dev
	leadID := createLead(t, pool, userID, "Даниил", "Floq", "dan@floq.dev")

	// Tier 1: name + company match
	pHigh := seedProspectWith(t, pool, userID, "Даниил", "Floq", "", "dan_tg")

	// Tier 2: name + email domain match (company differs)
	pMedium := seedProspectWith(t, pool, userID, "Даниил", "Другая компания", "daniil@floq.dev", "")

	// Tier 3: name only (no shared company, no shared domain)
	pLow := seedProspectWith(t, pool, userID, "Даниил", "TotallyDifferent", "other@example.com", "")

	// Non-matching: different name
	_ = seedProspectWith(t, pool, userID, "Иван", "Floq", "", "")

	// Should be excluded: status = converted
	pConverted := seedProspectWith(t, pool, userID, "Даниил", "Floq", "", "")
	// Convert pConverted directly to simulate existing link
	otherLeadID := createLead(t, pool, userID, "tmp", "", "")
	require.NoError(t, repo.ConvertToLead(ctx, pConverted.ID, otherLeadID))

	rows, err := repo.FindSuggestionsForLead(ctx, userID, leadID, "Даниил", "Floq", "dan@floq.dev")
	require.NoError(t, err)

	// Expect 3 tiers, ordered high → medium → low
	require.Len(t, rows, 3)
	assert.Equal(t, pHigh.ID, rows[0].ProspectID)
	assert.Equal(t, "high", rows[0].Confidence)
	assert.Equal(t, pMedium.ID, rows[1].ProspectID)
	assert.Equal(t, "medium", rows[1].Confidence)
	assert.Equal(t, pLow.ID, rows[2].ProspectID)
	assert.Equal(t, "low", rows[2].Confidence)
}

func TestFindSuggestionsForLead_NormalizationAndEdgeCases(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	leadID := createLead(t, pool, userID, "  Даниил  ", "  FLOQ  ", "dan@Floq.DEV")

	// Normalized name + company — still tier 1
	seedProspectWith(t, pool, userID, "даниил", "floq", "", "")

	rows, err := repo.FindSuggestionsForLead(ctx, userID, leadID, "  Даниил  ", "  FLOQ  ", "dan@Floq.DEV")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "high", rows[0].Confidence)
}

func TestFindSuggestionsForLead_EmptyCompanyNoTier1(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	// Both lead and prospect have empty company — should NOT be tier 1 (both empty != match).
	leadID := createLead(t, pool, userID, "Иван", "", "ivan@acme.com")
	seedProspectWith(t, pool, userID, "Иван", "", "ivan@acme.com", "")

	rows, err := repo.FindSuggestionsForLead(ctx, userID, leadID, "Иван", "", "ivan@acme.com")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	// Shared email domain → medium
	assert.Equal(t, "medium", rows[0].Confidence)
}

func TestFindSuggestionsForLead_NonOverlappingTiers(t *testing.T) {
	// A prospect that would satisfy BOTH tier 1 (name+company) AND tier 2
	// (name+email domain) must appear ONCE, classified as tier 1 — not
	// duplicated across tiers. This is guaranteed by the SQL CASE priority
	// ordering but worth pinning with a regression test.
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	leadID := createLead(t, pool, userID, "Даниил", "Floq", "dan@floq.dev")
	p := seedProspectWith(t, pool, userID, "Даниил", "Floq", "other@floq.dev", "")

	rows, err := repo.FindSuggestionsForLead(ctx, userID, leadID, "Даниил", "Floq", "dan@floq.dev")
	require.NoError(t, err)
	require.Len(t, rows, 1, "same prospect must not appear in multiple tiers")
	assert.Equal(t, p.ID, rows[0].ProspectID)
	assert.Equal(t, "high", rows[0].Confidence)
}

func TestFindSuggestionsForLead_EmptyNameReturnsNothing(t *testing.T) {
	// When the lead has no contact name we have no identifier to match on, so
	// the matcher must return nothing regardless of what prospects exist for
	// the user. (We don't seed an empty-name prospect because NewProspect
	// now rejects that as a domain-invariant violation.)
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	leadID := createLead(t, pool, userID, "", "Floq", "x@y.com")
	// Seed a valid prospect with a non-empty name to prove it's not a
	// "prospect missing" false negative — the empty lead-name is what drives
	// the empty result.
	seedProspectWith(t, pool, userID, "SomeName", "Floq", "x@y.com", "")

	rows, err := repo.FindSuggestionsForLead(ctx, userID, leadID, "", "Floq", "x@y.com")
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestFindSuggestionsForLead_ExcludesDismissed(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	leadID := createLead(t, pool, userID, "Даниил", "Floq", "dan@floq.dev")
	p := seedProspectWith(t, pool, userID, "Даниил", "Floq", "", "")

	// Before dismissal — present.
	rows, err := repo.FindSuggestionsForLead(ctx, userID, leadID, "Даниил", "Floq", "dan@floq.dev")
	require.NoError(t, err)
	require.Len(t, rows, 1)

	// Dismiss and re-query — excluded.
	require.NoError(t, repo.DismissSuggestion(ctx, leadID, p.ID))
	rows, err = repo.FindSuggestionsForLead(ctx, userID, leadID, "Даниил", "Floq", "dan@floq.dev")
	require.NoError(t, err)
	assert.Empty(t, rows)

	// Second dismiss is idempotent.
	assert.NoError(t, repo.DismissSuggestion(ctx, leadID, p.ID))
}

func TestCountSuggestionsForUser(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	lead1 := createLead(t, pool, userID, "Даниил", "Floq", "dan@floq.dev")
	lead2 := createLead(t, pool, userID, "Иван", "Acme", "ivan@acme.com")
	lead3 := createLead(t, pool, userID, "NoMatch", "X", "n@n.com")

	// Two prospects match lead1 by name
	seedProspectWith(t, pool, userID, "Даниил", "Floq", "", "")
	seedProspectWith(t, pool, userID, "Даниил", "Other", "", "")

	// One prospect matches lead2 by name
	p2 := seedProspectWith(t, pool, userID, "Иван", "Acme", "", "")

	counts, err := repo.CountSuggestionsForUser(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 2, counts[lead1])
	assert.Equal(t, 1, counts[lead2])
	assert.NotContains(t, counts, lead3)

	// Dismiss the only match for lead2 → it drops out of the map.
	require.NoError(t, repo.DismissSuggestion(ctx, lead2, p2.ID))
	counts, err = repo.CountSuggestionsForUser(ctx, userID)
	require.NoError(t, err)
	assert.NotContains(t, counts, lead2)
	assert.Equal(t, 2, counts[lead1])
}
