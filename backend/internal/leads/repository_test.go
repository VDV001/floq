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

func TestCreateAndGetLead(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	chatID := int64(123456)
	email := "lead@example.com"
	now := time.Now().UTC().Truncate(time.Microsecond)

	lead := &domain.Lead{
		ID:             uuid.New(),
		UserID:         userID,
		Channel:        domain.ChannelTelegram,
		ContactName:    "Alice",
		Company:        "ACME",
		FirstMessage:   "Hello",
		Status:         domain.StatusNew,
		TelegramChatID: &chatID,
		EmailAddress:   &email,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	err := repo.CreateLead(ctx, lead)
	require.NoError(t, err)

	got, err := repo.GetLead(ctx, lead.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, lead.ID, got.ID)
	assert.Equal(t, lead.ContactName, got.ContactName)
	assert.Equal(t, lead.Company, got.Company)
	assert.Equal(t, domain.StatusNew, got.Status)
	assert.Equal(t, &chatID, got.TelegramChatID)
	assert.Equal(t, &email, got.EmailAddress)
}

func TestGetLead_NotFound(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	got, err := repo.GetLead(ctx, uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestListLeads(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	for i := 0; i < 3; i++ {
		l := &domain.Lead{
			ID:          uuid.New(),
			UserID:      userID,
			Channel:     domain.ChannelEmail,
			ContactName: "Contact-" + uuid.New().String()[:8],
			Status:      domain.StatusNew,
			CreatedAt:   now.Add(time.Duration(i) * time.Second),
			UpdatedAt:   now,
		}
		require.NoError(t, repo.CreateLead(ctx, l))
	}

	list, err := repo.ListLeads(ctx, userID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 3)
}

func TestUpdateLeadStatus(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	lead := &domain.Lead{
		ID: uuid.New(), UserID: userID, Channel: domain.ChannelTelegram,
		ContactName: "Bob", Status: domain.StatusNew, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, repo.CreateLead(ctx, lead))

	err := repo.UpdateLeadStatus(ctx, lead.ID, domain.StatusQualified)
	require.NoError(t, err)

	got, err := repo.GetLead(ctx, lead.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusQualified, got.Status)
}

func TestUpdateFirstMessage(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	lead := &domain.Lead{
		ID: uuid.New(), UserID: userID, Channel: domain.ChannelEmail,
		ContactName: "Charlie", FirstMessage: "old", Status: domain.StatusNew,
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, repo.CreateLead(ctx, lead))

	err := repo.UpdateFirstMessage(ctx, lead.ID, "new message")
	require.NoError(t, err)

	got, err := repo.GetLead(ctx, lead.ID)
	require.NoError(t, err)
	assert.Equal(t, "new message", got.FirstMessage)
}

func TestCreateAndListMessages(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	lead := &domain.Lead{
		ID: uuid.New(), UserID: userID, Channel: domain.ChannelTelegram,
		ContactName: "Dave", Status: domain.StatusNew, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, repo.CreateLead(ctx, lead))

	msg1 := &domain.Message{
		ID: uuid.New(), LeadID: lead.ID, Direction: domain.DirectionInbound,
		Body: "hi there", SentAt: now,
	}
	msg2 := &domain.Message{
		ID: uuid.New(), LeadID: lead.ID, Direction: domain.DirectionOutbound,
		Body: "reply", SentAt: now.Add(time.Second),
	}

	require.NoError(t, repo.CreateMessage(ctx, msg1))
	require.NoError(t, repo.CreateMessage(ctx, msg2))

	msgs, err := repo.ListMessages(ctx, lead.ID)
	require.NoError(t, err)
	assert.Len(t, msgs, 2)
	assert.Equal(t, domain.DirectionInbound, msgs[0].Direction)
	assert.Equal(t, domain.DirectionOutbound, msgs[1].Direction)
}

func TestUpsertAndGetQualification(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	lead := &domain.Lead{
		ID: uuid.New(), UserID: userID, Channel: domain.ChannelEmail,
		ContactName: "Eve", Status: domain.StatusNew, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, repo.CreateLead(ctx, lead))

	q := &domain.Qualification{
		ID: uuid.New(), LeadID: lead.ID,
		IdentifiedNeed: "website", EstimatedBudget: "$5k", Deadline: "2 weeks",
		Score: 80, ScoreReason: "good fit", RecommendedAction: "call",
		ProviderUsed: "openai", GeneratedAt: now,
	}
	require.NoError(t, repo.UpsertQualification(ctx, q))

	got, err := repo.GetQualification(ctx, lead.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, q.IdentifiedNeed, got.IdentifiedNeed)
	assert.Equal(t, 80, got.Score)

	// Upsert again — should update, not duplicate
	q.Score = 95
	q.ScoreReason = "updated"
	require.NoError(t, repo.UpsertQualification(ctx, q))

	got2, err := repo.GetQualification(ctx, lead.ID)
	require.NoError(t, err)
	assert.Equal(t, 95, got2.Score)
	assert.Equal(t, "updated", got2.ScoreReason)
}

func TestGetQualification_NotFound(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := leads.NewRepository(pool)

	got, err := repo.GetQualification(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestCreateDraftAndGetLatest(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	lead := &domain.Lead{
		ID: uuid.New(), UserID: userID, Channel: domain.ChannelEmail,
		ContactName: "Frank", Status: domain.StatusNew, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, repo.CreateLead(ctx, lead))

	d1 := &domain.Draft{ID: uuid.New(), LeadID: lead.ID, Body: "draft 1", CreatedAt: now}
	d2 := &domain.Draft{ID: uuid.New(), LeadID: lead.ID, Body: "draft 2", CreatedAt: now.Add(time.Second)}

	require.NoError(t, repo.CreateDraft(ctx, d1))
	require.NoError(t, repo.CreateDraft(ctx, d2))

	got, err := repo.GetLatestDraft(ctx, lead.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "draft 2", got.Body)
}

func TestGetLatestDraft_NotFound(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := leads.NewRepository(pool)

	got, err := repo.GetLatestDraft(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestGetLeadByTelegramChatID(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	chatID := int64(999888777)
	now := time.Now().UTC().Truncate(time.Microsecond)
	lead := &domain.Lead{
		ID: uuid.New(), UserID: userID, Channel: domain.ChannelTelegram,
		ContactName: "Grace", Status: domain.StatusNew,
		TelegramChatID: &chatID, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, repo.CreateLead(ctx, lead))

	got, err := repo.GetLeadByTelegramChatID(ctx, userID, chatID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, lead.ID, got.ID)

	// Not found for different user
	got2, err := repo.GetLeadByTelegramChatID(ctx, uuid.New(), chatID)
	require.NoError(t, err)
	assert.Nil(t, got2)
}

func TestGetLeadByEmailAddress(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	email := "unique-" + uuid.New().String()[:8] + "@example.com"
	now := time.Now().UTC().Truncate(time.Microsecond)
	lead := &domain.Lead{
		ID: uuid.New(), UserID: userID, Channel: domain.ChannelEmail,
		ContactName: "Heidi", Status: domain.StatusNew,
		EmailAddress: &email, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, repo.CreateLead(ctx, lead))

	got, err := repo.GetLeadByEmailAddress(ctx, userID, email)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, lead.ID, got.ID)

	// Not found
	got2, err := repo.GetLeadByEmailAddress(ctx, userID, "nope@example.com")
	require.NoError(t, err)
	assert.Nil(t, got2)
}

func TestCountMonthAndTotalLeads(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	for i := 0; i < 5; i++ {
		l := &domain.Lead{
			ID: uuid.New(), UserID: userID, Channel: domain.ChannelEmail,
			ContactName: "Count-" + uuid.New().String()[:8], Status: domain.StatusNew,
			CreatedAt: now, UpdatedAt: now,
		}
		require.NoError(t, repo.CreateLead(ctx, l))
	}

	total, err := repo.CountTotalLeads(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 5, total)

	month, err := repo.CountMonthLeads(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 5, month)
}

func TestStaleLeadsWithoutReminder(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	lead := &domain.Lead{
		ID: uuid.New(), UserID: userID, Channel: domain.ChannelEmail,
		ContactName: "Stale", Status: domain.StatusNew,
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, repo.CreateLead(ctx, lead))

	// Add a message from 10 days ago
	oldMsg := &domain.Message{
		ID: uuid.New(), LeadID: lead.ID, Direction: domain.DirectionInbound,
		Body: "old message", SentAt: now.Add(-10 * 24 * time.Hour),
	}
	require.NoError(t, repo.CreateMessage(ctx, oldMsg))

	stale, err := repo.StaleLeadsWithoutReminder(ctx, 7)
	require.NoError(t, err)

	found := false
	for _, l := range stale {
		if l.ID == lead.ID {
			found = true
		}
	}
	assert.True(t, found, "expected stale lead to be returned")
}

func TestCreateReminder(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := leads.NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	lead := &domain.Lead{
		ID: uuid.New(), UserID: userID, Channel: domain.ChannelEmail,
		ContactName: "Reminder", Status: domain.StatusNew,
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, repo.CreateLead(ctx, lead))

	err := repo.CreateReminder(ctx, lead.ID, "follow up")
	require.NoError(t, err)
}
