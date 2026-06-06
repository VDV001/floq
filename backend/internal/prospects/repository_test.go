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

// TestConsentRoundTrip verifies the consent VO survives a persist→reload cycle
// through GetProspect for every consent state. Table-driven (≥3 variants):
// none carries no source/timestamp; obtained/withdrawn carry both.
func TestConsentRoundTrip(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	at := time.Now().UTC().Truncate(time.Microsecond)

	tests := []struct {
		name       string
		mutate     func(p *domain.Prospect)
		wantStatus domain.ConsentStatus
		wantSource string
		wantAtSet  bool
	}{
		{
			name:       "none is the cold default",
			mutate:     func(p *domain.Prospect) {},
			wantStatus: domain.ConsentStatusNone,
			wantSource: "",
			wantAtSet:  false,
		},
		{
			name:       "obtained carries source and timestamp",
			mutate:     func(p *domain.Prospect) { require.NoError(t, p.GrantConsent("inbound_reply", at)) },
			wantStatus: domain.ConsentStatusObtained,
			wantSource: "inbound_reply",
			wantAtSet:  true,
		},
		{
			name:       "withdrawn carries source and timestamp",
			mutate:     func(p *domain.Prospect) { require.NoError(t, p.WithdrawConsent("unsubscribe", at)) },
			wantStatus: domain.ConsentStatusWithdrawn,
			wantSource: "unsubscribe",
			wantAtSet:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newTestProspect(userID)
			tt.mutate(p)
			require.NoError(t, repo.CreateProspect(ctx, p))

			got, err := repo.GetProspect(ctx, p.ID)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantStatus, got.Consent.Status)
			assert.Equal(t, tt.wantSource, got.Consent.Source)
			if tt.wantAtSet {
				assert.False(t, got.Consent.Timestamp.IsZero(), "timestamp should be set")
			} else {
				assert.True(t, got.Consent.Timestamp.IsZero(), "timestamp should be zero for none")
			}
		})
	}
}

// TestConsentReadAcrossAllPaths guards every SELECT that hydrates a Prospect:
// each statement maintains its own column list, so a missing consent column in
// any one of them would silently drop the compliance state on that path.
func TestConsentReadAcrossAllPaths(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	at := time.Now().UTC().Truncate(time.Microsecond)
	p := newTestProspect(userID)
	require.NoError(t, p.GrantConsent("manual", at))
	require.NoError(t, repo.CreateProspect(ctx, p))

	t.Run("ListProspects", func(t *testing.T) {
		list, err := repo.ListProspects(ctx, userID)
		require.NoError(t, err)
		var found bool
		for _, item := range list {
			if item.ID == p.ID {
				found = true
				assert.Equal(t, domain.ConsentStatusObtained, item.Consent.Status)
				assert.Equal(t, "manual", item.Consent.Source)
			}
		}
		assert.True(t, found, "prospect not in list")
	})

	t.Run("GetProspectForUser", func(t *testing.T) {
		got, err := repo.GetProspectForUser(ctx, userID, p.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, domain.ConsentStatusObtained, got.Consent.Status)
		assert.Equal(t, "manual", got.Consent.Source)
	})

	t.Run("FindByEmail", func(t *testing.T) {
		got, err := repo.FindByEmail(ctx, userID, p.Email)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, domain.ConsentStatusObtained, got.Consent.Status)
	})

	t.Run("FindByTelegramUsername", func(t *testing.T) {
		got, err := repo.FindByTelegramUsername(ctx, userID, p.TelegramUsername)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, domain.ConsentStatusObtained, got.Consent.Status)
	})
}

// TestSuppression_AddAndCheck verifies the suppression list round-trips and
// that lookups are case-insensitive (via domain normalization) and scoped per
// channel.
func TestSuppression_AddAndCheck(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	ok, err := repo.IsSuppressed(ctx, userID, domain.SuppressionChannelEmail, "Bob@Example.com")
	require.NoError(t, err)
	require.False(t, ok, "should not be suppressed initially")

	s, err := domain.NewSuppression(userID, domain.SuppressionChannelEmail, "bob@example.com", "unsubscribe")
	require.NoError(t, err)
	require.NoError(t, repo.AddSuppression(ctx, s))

	ok, err = repo.IsSuppressed(ctx, userID, domain.SuppressionChannelEmail, "  BOB@example.COM ")
	require.NoError(t, err)
	require.True(t, ok, "should be suppressed after add, case-insensitively")

	ok, err = repo.IsSuppressed(ctx, userID, domain.SuppressionChannelTelegram, "bob@example.com")
	require.NoError(t, err)
	require.False(t, ok, "other channel must be independent")
}

// TestSuppression_Idempotent verifies a repeated suppression of the same
// address is a no-op, not a unique-constraint error.
func TestSuppression_Idempotent(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	s1, err := domain.NewSuppression(userID, domain.SuppressionChannelEmail, "dup@example.com", "unsubscribe")
	require.NoError(t, err)
	require.NoError(t, repo.AddSuppression(ctx, s1))

	s2, err := domain.NewSuppression(userID, domain.SuppressionChannelEmail, "dup@example.com", "manual")
	require.NoError(t, err)
	require.NoError(t, repo.AddSuppression(ctx, s2), "re-adding the same address must not conflict")

	ok, err := repo.IsSuppressed(ctx, userID, domain.SuppressionChannelEmail, "dup@example.com")
	require.NoError(t, err)
	require.True(t, ok)
}

// TestSuppression_TenantIsolation verifies suppression is scoped per user.
func TestSuppression_TenantIsolation(t *testing.T) {
	pool := testutil.TestDB(t)
	userA := testutil.SeedUser(t, pool)
	userB := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	s, err := domain.NewSuppression(userA, domain.SuppressionChannelEmail, "shared@example.com", "unsubscribe")
	require.NoError(t, err)
	require.NoError(t, repo.AddSuppression(ctx, s))

	ok, err := repo.IsSuppressed(ctx, userB, domain.SuppressionChannelEmail, "shared@example.com")
	require.NoError(t, err)
	require.False(t, ok, "another tenant must not see the suppression")
}

// TestUpdateConsent persists a withdrawal on an existing prospect and verifies
// it reloads, exercising the nullable consent_at mapping on the update path.
func TestUpdateConsent(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := prospects.NewRepository(pool)
	ctx := context.Background()

	p := newTestProspect(userID) // starts at consent 'none'
	require.NoError(t, repo.CreateProspect(ctx, p))

	at := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, p.WithdrawConsent("unsubscribe", at))
	require.NoError(t, repo.UpdateConsent(ctx, p.ID, p.Consent))

	got, err := repo.GetProspect(ctx, p.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, domain.ConsentStatusWithdrawn, got.Consent.Status)
	assert.Equal(t, "unsubscribe", got.Consent.Source)
	assert.False(t, got.Consent.Timestamp.IsZero())
}
