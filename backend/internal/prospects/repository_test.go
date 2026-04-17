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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestProspect(userID uuid.UUID) *domain.Prospect {
	p, _ := domain.NewProspect(userID, "Test "+uuid.New().String()[:8], "ACME", "CTO", "test-"+uuid.New().String()[:8]+"@example.com", "manual")
	p.TelegramUsername = "tg_" + uuid.New().String()[:8]
	return p
}

func TestCreateAndGetProspect(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	p := newTestProspect(userID)
	require.NoError(t, repo.CreateProspect(ctx, p))

	got, err := repo.GetProspect(ctx, p.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, p.ID, got.ID)
	assert.Equal(t, p.Name, got.Name)
	assert.Equal(t, p.Company, got.Company)
	assert.Equal(t, domain.ProspectStatusNew, got.Status)
	assert.Equal(t, domain.VerifyStatusNotChecked, got.VerifyStatus)
}

func TestGetProspect_NotFound(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := prospects.NewRepository(pool)

	got, err := repo.GetProspect(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestListProspects(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		require.NoError(t, repo.CreateProspect(ctx, newTestProspect(userID)))
	}

	list, err := repo.ListProspects(ctx, userID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 3)
}

func TestDeleteProspect(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	p := newTestProspect(userID)
	require.NoError(t, repo.CreateProspect(ctx, p))

	require.NoError(t, repo.DeleteProspect(ctx, p.ID))

	got, err := repo.GetProspect(ctx, p.ID)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestFindByEmail(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	p := newTestProspect(userID)
	require.NoError(t, repo.CreateProspect(ctx, p))

	got, err := repo.FindByEmail(ctx, userID, p.Email)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, p.ID, got.ID)

	// Case-insensitive
	got2, err := repo.FindByEmail(ctx, userID, "TEST-"+p.Email[5:])
	require.NoError(t, err)
	require.NotNil(t, got2)
	assert.Equal(t, p.ID, got2.ID)

	// Not found
	got3, err := repo.FindByEmail(ctx, userID, "nope@nope.com")
	require.NoError(t, err)
	assert.Nil(t, got3)
}

func TestFindByTelegramUsername(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	p := newTestProspect(userID)
	require.NoError(t, repo.CreateProspect(ctx, p))

	got, err := repo.FindByTelegramUsername(ctx, userID, p.TelegramUsername)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, p.ID, got.ID)

	// Not found
	got2, err := repo.FindByTelegramUsername(ctx, userID, "nonexistent_user")
	require.NoError(t, err)
	assert.Nil(t, got2)
}

func TestUpdateStatus(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	p := newTestProspect(userID)
	require.NoError(t, repo.CreateProspect(ctx, p))

	require.NoError(t, repo.UpdateStatus(ctx, p.ID, domain.ProspectStatusInSequence))

	got, err := repo.GetProspect(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ProspectStatusInSequence, got.Status)
}

func TestConvertToLead(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	p := newTestProspect(userID)
	require.NoError(t, repo.CreateProspect(ctx, p))

	// Create a lead for FK reference
	now := time.Now().UTC().Truncate(time.Microsecond)
	leadID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO leads (id, user_id, channel, contact_name, company, first_message, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		leadID, userID, "email", "Converted", "Co", "msg", "new", now, now)
	require.NoError(t, err)

	require.NoError(t, repo.ConvertToLead(ctx, p.ID, leadID))

	got, err := repo.GetProspect(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ProspectStatusConverted, got.Status)
	require.NotNil(t, got.ConvertedLeadID)
	assert.Equal(t, leadID, *got.ConvertedLeadID)
}

func TestUpdateVerification(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	p := newTestProspect(userID)
	require.NoError(t, repo.CreateProspect(ctx, p))

	now := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, repo.UpdateVerification(ctx, p.ID, domain.VerifyStatusValid, 95, `{"ok":true}`, now))

	got, err := repo.GetProspect(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.VerifyStatusValid, got.VerifyStatus)
	assert.Equal(t, 95, got.VerifyScore)
	require.NotNil(t, got.VerifiedAt)
}

func TestCreateProspectsBatch(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	batch := make([]domain.Prospect, 3)
	for i := range batch {
		p := newTestProspect(userID)
		batch[i] = *p
	}

	require.NoError(t, repo.CreateProspectsBatch(ctx, batch))

	list, err := repo.ListProspects(ctx, userID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 3)
}
