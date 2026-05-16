//go:build integration

package leads_test

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/leads"
	"github.com/daniil/floq/internal/leads/domain"
	"github.com/daniil/floq/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentityRepository_SaveAndFindByEmail(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewIdentityRepository(pool)
	ctx := context.Background()

	id, err := domain.NewIdentity(userID, "alice@acme.com", "+79991234567", "alice_bot")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, id))

	got, err := repo.FindByEmail(ctx, userID, "alice@acme.com")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, id.ID, got.ID)
	assert.Equal(t, "alice@acme.com", got.Email)
	assert.Equal(t, "+79991234567", got.Phone)
	assert.Equal(t, "alice_bot", got.TelegramUsername)
}

func TestIdentityRepository_FindByPhone_AndTelegram(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewIdentityRepository(pool)
	ctx := context.Background()

	id, err := domain.NewIdentity(userID, "", "+79991234567", "alice_bot")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, id))

	byPhone, err := repo.FindByPhone(ctx, userID, "+79991234567")
	require.NoError(t, err)
	require.NotNil(t, byPhone)
	assert.Equal(t, id.ID, byPhone.ID)

	byTg, err := repo.FindByTelegramUsername(ctx, userID, "alice_bot")
	require.NoError(t, err)
	require.NotNil(t, byTg)
	assert.Equal(t, id.ID, byTg.ID)
}

func TestIdentityRepository_FindByEmail_NotFound(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewIdentityRepository(pool)
	ctx := context.Background()

	got, err := repo.FindByEmail(ctx, userID, "missing@example.com")
	require.NoError(t, err, "missing row is not an error")
	assert.Nil(t, got)
}

func TestIdentityRepository_UniqueEmailPerUser(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewIdentityRepository(pool)
	ctx := context.Background()

	first, err := domain.NewIdentity(userID, "alice@acme.com", "", "")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, first))

	second, err := domain.NewIdentity(userID, "alice@acme.com", "+79991234567", "")
	require.NoError(t, err)
	err = repo.Save(ctx, second)
	require.Error(t, err, "partial unique index must reject duplicate email per user")
}

func TestIdentityRepository_ScopedByUser(t *testing.T) {
	pool := testutil.TestDB(t)
	userA := testutil.SeedUser(t, pool)
	userB := testutil.SeedUser(t, pool)
	repo := leads.NewIdentityRepository(pool)
	ctx := context.Background()

	idA, err := domain.NewIdentity(userA, "alice@acme.com", "", "")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, idA))

	got, err := repo.FindByEmail(ctx, userB, "alice@acme.com")
	require.NoError(t, err)
	assert.Nil(t, got, "user B must not see user A's identity even with identical email")
}
