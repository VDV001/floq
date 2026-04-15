//go:build integration

package sequences_test

import (
	"context"
	"testing"
	"time"

	pdomain "github.com/daniil/floq/internal/prospects/domain"

	"github.com/daniil/floq/internal/sequences"
	"github.com/daniil/floq/internal/sequences/domain"
	"github.com/daniil/floq/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAndGetSequence(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sequences.NewRepository(pool)
	ctx := context.Background()

	seq := domain.NewSequence(userID, "Test Sequence")

	err := repo.CreateSequence(ctx, seq)
	require.NoError(t, err)

	got, err := repo.GetSequence(ctx, seq.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, seq.Name, got.Name)
	assert.Equal(t, false, got.IsActive)
}

func TestGetSequence_NotFound(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := sequences.NewRepository(pool)

	got, err := repo.GetSequence(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestListSequences(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sequences.NewRepository(pool)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		s := domain.NewSequence(userID, "Seq-"+uuid.New().String()[:8])
		require.NoError(t, repo.CreateSequence(ctx, s))
	}

	list, err := repo.ListSequences(ctx, userID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 3)
}

func TestUpdateSequence(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sequences.NewRepository(pool)
	ctx := context.Background()

	seq := domain.NewSequence(userID, "Original")
	require.NoError(t, repo.CreateSequence(ctx, seq))

	seq.Name = "Updated"
	require.NoError(t, repo.UpdateSequence(ctx, seq))

	got, err := repo.GetSequence(ctx, seq.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", got.Name)
}

func TestDeleteSequence(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sequences.NewRepository(pool)
	ctx := context.Background()

	seq := domain.NewSequence(userID, "ToDelete")
	require.NoError(t, repo.CreateSequence(ctx, seq))

	require.NoError(t, repo.DeleteSequence(ctx, seq.ID))

	got, err := repo.GetSequence(ctx, seq.ID)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestToggleActive(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sequences.NewRepository(pool)
	ctx := context.Background()

	seq := domain.NewSequence(userID, "Toggle")
	require.NoError(t, repo.CreateSequence(ctx, seq))

	require.NoError(t, repo.ToggleActive(ctx, seq.ID, true))
	got, err := repo.GetSequence(ctx, seq.ID)
	require.NoError(t, err)
	assert.True(t, got.IsActive)

	require.NoError(t, repo.ToggleActive(ctx, seq.ID, false))
	got, err = repo.GetSequence(ctx, seq.ID)
	require.NoError(t, err)
	assert.False(t, got.IsActive)
}

func TestCreateAndListSteps(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sequences.NewRepository(pool)
	ctx := context.Background()

	seq := domain.NewSequence(userID, "WithSteps")
	require.NoError(t, repo.CreateSequence(ctx, seq))

	step1 := domain.NewSequenceStep(seq.ID, 1, 0, domain.StepChannelEmail, "intro")
	step2 := domain.NewSequenceStep(seq.ID, 2, 3, domain.StepChannelTelegram, "followup")

	require.NoError(t, repo.CreateStep(ctx, step1))
	require.NoError(t, repo.CreateStep(ctx, step2))

	steps, err := repo.ListSteps(ctx, seq.ID)
	require.NoError(t, err)
	assert.Len(t, steps, 2)
	assert.Equal(t, 1, steps[0].StepOrder)
	assert.Equal(t, 2, steps[1].StepOrder)
}

func TestDeleteStep(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sequences.NewRepository(pool)
	ctx := context.Background()

	seq := domain.NewSequence(userID, "DelStep")
	require.NoError(t, repo.CreateSequence(ctx, seq))

	step := domain.NewSequenceStep(seq.ID, 1, 0, domain.StepChannelEmail, "to delete")
	require.NoError(t, repo.CreateStep(ctx, step))

	require.NoError(t, repo.DeleteStep(ctx, step.ID))

	steps, err := repo.ListSteps(ctx, seq.ID)
	require.NoError(t, err)
	assert.Empty(t, steps)
}

// seedProspect creates a prospect for outbound message tests.
func seedProspect(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	p := pdomain.NewProspect(userID, "Test Prospect", "Co", "CTO", "p@example.com", "manual")
	_, err := pool.Exec(ctx,
		`INSERT INTO prospects (id, user_id, name, company, title, email, phone, whatsapp, telegram_username, industry, company_size, context,
		                        source, source_id, status, verify_status, verify_score, verify_details, verified_at, converted_lead_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)`,
		p.ID, p.UserID, p.Name, p.Company, p.Title, p.Email, p.Phone, p.WhatsApp, p.TelegramUsername, p.Industry, p.CompanySize, p.Context,
		p.Source, p.SourceID, p.Status, p.VerifyStatus, p.VerifyScore, p.VerifyDetails, p.VerifiedAt, p.ConvertedLeadID, p.CreatedAt, p.UpdatedAt)
	require.NoError(t, err)
	return p.ID
}

func TestCreateOutboundAndListQueue(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sequences.NewRepository(pool)
	ctx := context.Background()

	seq := domain.NewSequence(userID, "Outbound Seq")
	require.NoError(t, repo.CreateSequence(ctx, seq))

	prospectID := seedProspect(t, pool, userID)

	msg := domain.NewOutboundMessage(prospectID, seq.ID, 1, domain.StepChannelEmail, "Hello!", time.Now().UTC())

	require.NoError(t, repo.CreateOutboundMessage(ctx, msg))

	queue, err := repo.ListOutboundQueue(ctx, userID)
	require.NoError(t, err)

	found := false
	for _, m := range queue {
		if m.ID == msg.ID {
			found = true
			assert.Equal(t, domain.OutboundStatusDraft, m.Status)
		}
	}
	assert.True(t, found)
}

func TestApproveAndListSent(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sequences.NewRepository(pool)
	ctx := context.Background()

	seq := domain.NewSequence(userID, "ApproveSeq")
	require.NoError(t, repo.CreateSequence(ctx, seq))
	prospectID := seedProspect(t, pool, userID)

	msg := domain.NewOutboundMessage(prospectID, seq.ID, 1, domain.StepChannelEmail, "body", time.Now().UTC())
	require.NoError(t, repo.CreateOutboundMessage(ctx, msg))

	// Approve
	require.NoError(t, repo.UpdateOutboundStatus(ctx, msg.ID, domain.OutboundStatusApproved))

	sent, err := repo.ListSentMessages(ctx, userID)
	require.NoError(t, err)
	found := false
	for _, m := range sent {
		if m.ID == msg.ID {
			found = true
			assert.Equal(t, domain.OutboundStatusApproved, m.Status)
		}
	}
	assert.True(t, found)
}

func TestUpdateOutboundBody(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sequences.NewRepository(pool)
	ctx := context.Background()

	seq := domain.NewSequence(userID, "BodyUpd")
	require.NoError(t, repo.CreateSequence(ctx, seq))
	prospectID := seedProspect(t, pool, userID)

	msg := domain.NewOutboundMessage(prospectID, seq.ID, 1, domain.StepChannelEmail, "old body", time.Now().UTC())
	require.NoError(t, repo.CreateOutboundMessage(ctx, msg))

	require.NoError(t, repo.UpdateOutboundBody(ctx, msg.ID, "new body"))

	got, err := repo.GetOutboundMessage(ctx, msg.ID)
	require.NoError(t, err)
	assert.Equal(t, "new body", got.Body)
}

func TestGetOutboundMessage_NotFound(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := sequences.NewRepository(pool)

	got, err := repo.GetOutboundMessage(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMarkSentAndOpened(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sequences.NewRepository(pool)
	ctx := context.Background()

	seq := domain.NewSequence(userID, "MarkSeq")
	require.NoError(t, repo.CreateSequence(ctx, seq))
	require.NoError(t, repo.ToggleActive(ctx, seq.ID, true))
	prospectID := seedProspect(t, pool, userID)

	msg := domain.NewOutboundMessage(prospectID, seq.ID, 1, domain.StepChannelEmail, "body", time.Now().UTC().Add(-time.Hour))
	require.NoError(t, repo.CreateOutboundMessage(ctx, msg))
	require.NoError(t, repo.UpdateOutboundStatus(ctx, msg.ID, domain.OutboundStatusApproved))

	// GetPendingSends — should find it
	pending, err := repo.GetPendingSends(ctx)
	require.NoError(t, err)
	found := false
	for _, m := range pending {
		if m.ID == msg.ID {
			found = true
		}
	}
	assert.True(t, found, "expected message in pending sends")

	// MarkSent
	require.NoError(t, repo.MarkSent(ctx, msg.ID))
	got, err := repo.GetOutboundMessage(ctx, msg.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.OutboundStatusSent, got.Status)
	assert.NotNil(t, got.SentAt)

	// MarkOpened
	require.NoError(t, repo.MarkOpened(ctx, msg.ID))
}

func TestMarkBounced(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sequences.NewRepository(pool)
	ctx := context.Background()

	seq := domain.NewSequence(userID, "BounceSeq")
	require.NoError(t, repo.CreateSequence(ctx, seq))
	prospectID := seedProspect(t, pool, userID)

	msg := domain.NewOutboundMessage(prospectID, seq.ID, 1, domain.StepChannelEmail, "body", time.Now().UTC())
	require.NoError(t, repo.CreateOutboundMessage(ctx, msg))

	require.NoError(t, repo.MarkBounced(ctx, msg.ID))

	got, err := repo.GetOutboundMessage(ctx, msg.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.OutboundStatusBounced, got.Status)
}

func TestMarkRepliedByProspect(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sequences.NewRepository(pool)
	ctx := context.Background()

	seq := domain.NewSequence(userID, "ReplySeq")
	require.NoError(t, repo.CreateSequence(ctx, seq))
	prospectID := seedProspect(t, pool, userID)

	msg := domain.NewOutboundMessage(prospectID, seq.ID, 1, domain.StepChannelEmail, "body", time.Now().UTC())
	require.NoError(t, repo.CreateOutboundMessage(ctx, msg))
	require.NoError(t, repo.MarkSent(ctx, msg.ID))

	require.NoError(t, repo.MarkRepliedByProspect(ctx, prospectID))
}

func TestSaveAndGetRecentFeedback(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sequences.NewRepository(pool)
	ctx := context.Background()

	require.NoError(t, repo.SavePromptFeedback(ctx, userID, "original", "edited", "ctx", "email"))
	require.NoError(t, repo.SavePromptFeedback(ctx, userID, "orig2", "edit2", "ctx2", "telegram"))

	fb, err := repo.GetRecentFeedback(ctx, userID, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(fb), 2)
}

func TestGetConversationHistory(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sequences.NewRepository(pool)
	ctx := context.Background()

	seq := domain.NewSequence(userID, "ConvSeq")
	require.NoError(t, repo.CreateSequence(ctx, seq))
	prospectID := seedProspect(t, pool, userID)

	msg := domain.NewOutboundMessage(prospectID, seq.ID, 1, domain.StepChannelEmail, "conv body", time.Now().UTC())
	require.NoError(t, repo.CreateOutboundMessage(ctx, msg))
	require.NoError(t, repo.MarkSent(ctx, msg.ID))

	entries, err := repo.GetConversationHistory(ctx, prospectID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(entries), 1)
	assert.Equal(t, "conv body", entries[0].Body)
}

func TestGetStats(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	repo := sequences.NewRepository(pool)
	ctx := context.Background()

	seq := domain.NewSequence(userID, "StatsSeq")
	require.NoError(t, repo.CreateSequence(ctx, seq))
	prospectID := seedProspect(t, pool, userID)

	// Create 2 messages: one draft, one sent
	m1 := domain.NewOutboundMessage(prospectID, seq.ID, 1, domain.StepChannelEmail, "draft", time.Now().UTC())
	m2 := domain.NewOutboundMessage(prospectID, seq.ID, 2, domain.StepChannelEmail, "sent", time.Now().UTC())
	require.NoError(t, repo.CreateOutboundMessage(ctx, m1))
	require.NoError(t, repo.CreateOutboundMessage(ctx, m2))
	require.NoError(t, repo.MarkSent(ctx, m2.ID))

	stats, err := repo.GetStats(ctx, userID)
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, 1, stats.Draft)
	assert.Equal(t, 1, stats.Sent)
}
