//go:build integration

package sources_test

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/sources"
	"github.com/daniil/floq/internal/sources/domain"
	"github.com/daniil/floq/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAndListCategories(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sources.NewRepository(pool)
	ctx := context.Background()

	cat1, err := domain.NewCategory(userID, "Cat-"+uuid.New().String()[:8])
	require.NoError(t, err)
	cat2, err := domain.NewCategory(userID, "Cat-"+uuid.New().String()[:8])
	require.NoError(t, err)

	require.NoError(t, repo.CreateCategory(ctx, cat1))
	require.NoError(t, repo.CreateCategory(ctx, cat2))

	cats, err := repo.ListCategories(ctx, userID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(cats), 2)

	// Categories should have empty Sources slice, not nil
	for _, c := range cats {
		assert.NotNil(t, c.Sources)
	}
}

func TestUpdateCategory(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sources.NewRepository(pool)
	ctx := context.Background()

	cat, err := domain.NewCategory(userID, "Original-"+uuid.New().String()[:8])
	require.NoError(t, err)
	require.NoError(t, repo.CreateCategory(ctx, cat))

	newName := "Renamed-" + uuid.New().String()[:8]
	require.NoError(t, repo.UpdateCategory(ctx, cat.ID, newName))

	// Verify via ListCategories
	cats, err := repo.ListCategories(ctx, userID)
	require.NoError(t, err)
	found := false
	for _, c := range cats {
		if c.ID == cat.ID {
			assert.Equal(t, newName, c.Name)
			found = true
		}
	}
	assert.True(t, found)
}

func TestDeleteCategory(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sources.NewRepository(pool)
	ctx := context.Background()

	cat, err := domain.NewCategory(userID, "ToDelete-"+uuid.New().String()[:8])
	require.NoError(t, err)
	require.NoError(t, repo.CreateCategory(ctx, cat))

	require.NoError(t, repo.DeleteCategory(ctx, cat.ID))

	cats, err := repo.ListCategories(ctx, userID)
	require.NoError(t, err)
	for _, c := range cats {
		assert.NotEqual(t, cat.ID, c.ID)
	}
}

func TestCreateAndGetSource(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sources.NewRepository(pool)
	ctx := context.Background()

	cat, err := domain.NewCategory(userID, "SrcCat-"+uuid.New().String()[:8])
	require.NoError(t, err)
	require.NoError(t, repo.CreateCategory(ctx, cat))

	src, err := domain.NewSource(userID, cat.ID, "Source-"+uuid.New().String()[:8])
	require.NoError(t, err)
	require.NoError(t, repo.CreateSource(ctx, src))

	got, err := repo.GetSource(ctx, src.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, src.ID, got.ID)
	assert.Equal(t, src.Name, got.Name)
	assert.Equal(t, cat.ID, got.CategoryID)
}

func TestGetSource_NotFound(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := sources.NewRepository(pool)

	got, err := repo.GetSource(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestUpdateSource(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sources.NewRepository(pool)
	ctx := context.Background()

	cat, err := domain.NewCategory(userID, "UpdSrcCat-"+uuid.New().String()[:8])
	require.NoError(t, err)
	require.NoError(t, repo.CreateCategory(ctx, cat))

	src, err := domain.NewSource(userID, cat.ID, "OldName-"+uuid.New().String()[:8])
	require.NoError(t, err)
	require.NoError(t, repo.CreateSource(ctx, src))

	newName := "NewName-" + uuid.New().String()[:8]
	require.NoError(t, repo.UpdateSource(ctx, src.ID, newName))

	got, err := repo.GetSource(ctx, src.ID)
	require.NoError(t, err)
	assert.Equal(t, newName, got.Name)
}

func TestDeleteSource(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sources.NewRepository(pool)
	ctx := context.Background()

	cat, err := domain.NewCategory(userID, "DelSrcCat-"+uuid.New().String()[:8])
	require.NoError(t, err)
	require.NoError(t, repo.CreateCategory(ctx, cat))

	src, err := domain.NewSource(userID, cat.ID, "ToDelete-"+uuid.New().String()[:8])
	require.NoError(t, err)
	require.NoError(t, repo.CreateSource(ctx, src))

	require.NoError(t, repo.DeleteSource(ctx, src.ID))

	got, err := repo.GetSource(ctx, src.ID)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestListCategoriesWithSources(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sources.NewRepository(pool)
	ctx := context.Background()

	cat, err := domain.NewCategory(userID, "WithSrc-"+uuid.New().String()[:8])
	require.NoError(t, err)
	require.NoError(t, repo.CreateCategory(ctx, cat))

	src1, err := domain.NewSource(userID, cat.ID, "Src1-"+uuid.New().String()[:8])
	require.NoError(t, err)
	src2, err := domain.NewSource(userID, cat.ID, "Src2-"+uuid.New().String()[:8])
	require.NoError(t, err)
	require.NoError(t, repo.CreateSource(ctx, src1))
	require.NoError(t, repo.CreateSource(ctx, src2))

	cats, err := repo.ListCategories(ctx, userID)
	require.NoError(t, err)

	found := false
	for _, c := range cats {
		if c.ID == cat.ID {
			found = true
			assert.Len(t, c.Sources, 2)
		}
	}
	assert.True(t, found)
}

func TestEnsureDefaults(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sources.NewRepository(pool)
	ctx := context.Background()

	// First call creates defaults
	require.NoError(t, repo.EnsureDefaults(ctx, userID))

	cats, err := repo.ListCategories(ctx, userID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(cats), 4) // 4 default categories

	// Second call should be a no-op
	require.NoError(t, repo.EnsureDefaults(ctx, userID))
}

func TestSourceStats(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sources.NewRepository(pool)
	ctx := context.Background()

	cat, err := domain.NewCategory(userID, "StatsCat-"+uuid.New().String()[:8])
	require.NoError(t, err)
	require.NoError(t, repo.CreateCategory(ctx, cat))

	src, err := domain.NewSource(userID, cat.ID, "StatsSrc-"+uuid.New().String()[:8])
	require.NoError(t, err)
	require.NoError(t, repo.CreateSource(ctx, src))

	stats, err := repo.SourceStats(ctx, userID)
	require.NoError(t, err)

	found := false
	for _, s := range stats {
		if s.SourceID == src.ID {
			found = true
			assert.Equal(t, 0, s.ProspectCount)
			assert.Equal(t, 0, s.LeadCount)
		}
	}
	assert.True(t, found)
}

func TestSourceStats_ExcludesArchivedLeads(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sources.NewRepository(pool)
	ctx := context.Background()

	cat, err := domain.NewCategory(userID, "ArchCat-"+uuid.New().String()[:8])
	require.NoError(t, err)
	require.NoError(t, repo.CreateCategory(ctx, cat))
	src, err := domain.NewSource(userID, cat.ID, "ArchSrc-"+uuid.New().String()[:8])
	require.NoError(t, err)
	require.NoError(t, repo.CreateSource(ctx, src))

	// One active lead, one archived lead, both attributed to this source.
	seedSourcedLead(t, pool, userID, src.ID, false)
	seedSourcedLead(t, pool, userID, src.ID, true)

	stats, err := repo.SourceStats(ctx, userID)
	require.NoError(t, err)

	for _, s := range stats {
		if s.SourceID == src.ID {
			assert.Equal(t, 1, s.LeadCount, "archived lead must not be counted in source stats")
		}
	}
}

func seedSourcedLead(t *testing.T, pool *pgxpool.Pool, userID, sourceID uuid.UUID, archived bool) {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO leads (id, user_id, channel, contact_name, first_message, status, source_id, created_at, updated_at)
		 VALUES ($1, $2, 'email', 'c', 'hi', 'new', $3, now(), now())`,
		id, userID, sourceID)
	require.NoError(t, err, "seed sourced lead")
	if archived {
		_, err = pool.Exec(context.Background(), `UPDATE leads SET archived_at = now() WHERE id = $1`, id)
		require.NoError(t, err, "archive lead")
	}
}
