//go:build integration

package enrichment_test

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/enrichment"
	"github.com/daniil/floq/internal/enrichment/domain"
	"github.com/daniil/floq/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustDomain(t *testing.T, email string) domain.Domain {
	t.Helper()
	d, err := domain.NewDomain(email)
	require.NoError(t, err)
	return d
}

func mustPending(t *testing.T, userID uuid.UUID, email string) *domain.CompanyEnrichment {
	t.Helper()
	e, err := domain.NewPendingEnrichment(userID, mustDomain(t, email))
	require.NoError(t, err)
	return e
}

func TestRepository_UpsertPending_Idempotent(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := enrichment.NewRepository(pool)

	require.NoError(t, repo.UpsertPending(ctx, mustPending(t, userID, "ivan@acme-upsert.ru")))
	// Second upsert for the same (user, domain) must not duplicate or error.
	require.NoError(t, repo.UpsertPending(ctx, mustPending(t, userID, "sales@acme-upsert.ru")))

	got, found, err := repo.Get(ctx, userID, "acme-upsert.ru")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, domain.StatusPending, got.Status)
}

func TestRepository_Get_TenantScoped(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	owner := testutil.SeedUser(t, pool)
	other := testutil.SeedUser(t, pool)
	repo := enrichment.NewRepository(pool)

	require.NoError(t, repo.UpsertPending(ctx, mustPending(t, owner, "ivan@acme-scope.ru")))

	_, found, err := repo.Get(ctx, owner, "acme-scope.ru")
	require.NoError(t, err)
	assert.True(t, found, "owner sees their enrichment")

	_, found, err = repo.Get(ctx, other, "acme-scope.ru")
	require.NoError(t, err)
	assert.False(t, found, "another tenant must NOT see it")
}

func TestRepository_Save_RoundTripsProfile(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := enrichment.NewRepository(pool)

	e := mustPending(t, userID, "ivan@acme-save.ru")
	require.NoError(t, repo.UpsertPending(ctx, e))

	stored, _, err := repo.Get(ctx, userID, "acme-save.ru")
	require.NoError(t, err)
	profile := domain.CompanyProfile{Title: "Acme", Description: "Widgets", Emails: []string{"info@acme-save.ru"}, Socials: []string{"https://t.me/acme"}}
	stored.MarkEnriched(profile, 3600)
	require.NoError(t, repo.Save(ctx, stored))

	got, found, err := repo.Get(ctx, userID, "acme-save.ru")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, domain.StatusEnriched, got.Status)
	assert.Equal(t, profile, got.Profile)
	require.NotNil(t, got.EnrichedAt)
	require.NotNil(t, got.ExpiresAt)
}

func TestRepository_ClaimDue_ReturnsPendingNotFresh(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := enrichment.NewRepository(pool)

	// One pending (due) and one freshly-enriched (not due) for this user.
	require.NoError(t, repo.UpsertPending(ctx, mustPending(t, userID, "ivan@due-pending.ru")))
	fresh := mustPending(t, userID, "ivan@fresh-enriched.ru")
	require.NoError(t, repo.UpsertPending(ctx, fresh))
	stored, _, _ := repo.Get(ctx, userID, "fresh-enriched.ru")
	stored.MarkEnriched(domain.CompanyProfile{Title: "Fresh"}, 3600) // expires in 1h → not due
	require.NoError(t, repo.Save(ctx, stored))

	due, err := repo.ClaimDue(ctx, 100, 3)
	require.NoError(t, err)

	domains := map[string]bool{}
	for _, e := range due {
		domains[e.Domain.String()] = true
	}
	assert.True(t, domains["due-pending.ru"], "pending row is claimed")
	assert.False(t, domains["fresh-enriched.ru"], "freshly-enriched row is NOT claimed")
}

func TestRepository_ClaimDue_RespectsMaxAttempts(t *testing.T) {
	ctx := context.Background()
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := enrichment.NewRepository(pool)

	e := mustPending(t, userID, "ivan@exhausted.ru")
	require.NoError(t, repo.UpsertPending(ctx, e))
	stored, _, _ := repo.Get(ctx, userID, "exhausted.ru")
	stored.MarkFailed("boom")
	stored.MarkFailed("boom")
	stored.MarkFailed("boom") // attempts = 3
	require.NoError(t, repo.Save(ctx, stored))

	due, err := repo.ClaimDue(ctx, 100, 3)
	require.NoError(t, err)
	for _, d := range due {
		assert.NotEqual(t, "exhausted.ru", d.Domain.String(), "attempts==max is not re-claimed")
	}
}
