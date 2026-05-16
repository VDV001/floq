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
	assert.WithinDuration(t, id.CreatedAt, got.CreatedAt, time.Second, "CreatedAt must round-trip")
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

func TestIdentityRepository_LinkLead_AndIsIdempotent(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewIdentityRepository(pool)
	ctx := context.Background()

	leadID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO leads (id, user_id, channel, contact_name, first_message) VALUES ($1, $2, 'email', 'Alice', 'hi')`,
		leadID, userID)
	require.NoError(t, err)

	id, err := domain.NewIdentity(userID, "alice@acme.com", "", "")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, id))

	require.NoError(t, repo.LinkLead(ctx, leadID, id.ID))
	require.NoError(t, repo.LinkLead(ctx, leadID, id.ID), "duplicate LinkLead must not error")

	var cnt int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM lead_identities WHERE lead_id = $1 AND identity_id = $2`,
		leadID, id.ID).Scan(&cnt))
	assert.Equal(t, 1, cnt, "ON CONFLICT DO NOTHING must keep the link table at one row")
}

func TestIdentityRepository_LinkProspect_AndIsIdempotent(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewIdentityRepository(pool)
	ctx := context.Background()

	prospectID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO prospects (id, user_id, name, email) VALUES ($1, $2, 'Alice', 'alice@acme.com')`,
		prospectID, userID)
	require.NoError(t, err)

	id, err := domain.NewIdentity(userID, "alice@acme.com", "", "")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, id))

	require.NoError(t, repo.LinkProspect(ctx, prospectID, id.ID))
	require.NoError(t, repo.LinkProspect(ctx, prospectID, id.ID), "duplicate LinkProspect must not error")

	var cnt int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM prospect_identities WHERE prospect_id = $1 AND identity_id = $2`,
		prospectID, id.ID).Scan(&cnt))
	assert.Equal(t, 1, cnt)
}

func TestIdentityRepository_Link_RejectsMissingFK(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewIdentityRepository(pool)
	ctx := context.Background()

	id, err := domain.NewIdentity(userID, "alice@acme.com", "", "")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, id))

	err = repo.LinkLead(ctx, uuid.New(), id.ID)
	require.Error(t, err, "FK on lead_id must reject inserts pointing at a non-existent lead")
}

func TestIdentityRepository_GetByLeadID_ReturnsLinkedIdentity(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewIdentityRepository(pool)
	ctx := context.Background()

	leadID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO leads (id, user_id, channel, contact_name, first_message) VALUES ($1, $2, 'email', 'Alice', 'hi')`,
		leadID, userID)
	require.NoError(t, err)

	id, err := domain.NewIdentity(userID, "alice@acme.com", "+79991234567", "alice_bot")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, id))
	require.NoError(t, repo.LinkLead(ctx, leadID, id.ID))

	got, err := repo.GetByLeadID(ctx, leadID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, id.ID, got.ID)
	assert.Equal(t, "alice@acme.com", got.Email)
	assert.Equal(t, "+79991234567", got.Phone)
	assert.Equal(t, "alice_bot", got.TelegramUsername)
}

func TestIdentityRepository_GetByLeadID_NoLink_ReturnsNil(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewIdentityRepository(pool)
	ctx := context.Background()

	leadID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO leads (id, user_id, channel, contact_name, first_message) VALUES ($1, $2, 'email', 'Alice', 'hi')`,
		leadID, userID)
	require.NoError(t, err)

	got, err := repo.GetByLeadID(ctx, leadID)
	require.NoError(t, err, "missing link is not an error")
	assert.Nil(t, got)
}

func TestIdentityRepository_LinkedLeadIDs(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewIdentityRepository(pool)
	ctx := context.Background()

	leadA, leadB := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{leadA, leadB} {
		_, err := pool.Exec(ctx,
			`INSERT INTO leads (id, user_id, channel, contact_name, first_message) VALUES ($1, $2, 'email', 'Alice', 'hi')`,
			id, userID)
		require.NoError(t, err)
	}

	id, err := domain.NewIdentity(userID, "alice@acme.com", "", "")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, id))
	require.NoError(t, repo.LinkLead(ctx, leadA, id.ID))
	require.NoError(t, repo.LinkLead(ctx, leadB, id.ID))

	leadIDs, err := repo.LinkedLeadIDs(ctx, id.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []uuid.UUID{leadA, leadB}, leadIDs)
}

func TestIdentityRepository_LinkedLeadIDs_None(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewIdentityRepository(pool)
	ctx := context.Background()

	id, err := domain.NewIdentity(userID, "alice@acme.com", "", "")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, id))

	leadIDs, err := repo.LinkedLeadIDs(ctx, id.ID)
	require.NoError(t, err)
	assert.Empty(t, leadIDs, "identity without any LinkLead must return an empty slice")
}
