package leads

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auditdomain "github.com/daniil/floq/internal/audit/domain"
	"github.com/daniil/floq/internal/leads/domain"
)

// auditingAI is a domain.AIService that captures the ctx of each call
// so tests can assert CallMeta attribution downstream of the use case.
// It returns trivial happy-path responses — these tests are about
// attribution, not the qualify/draft results themselves.
type auditingAI struct {
	qualifyCtx context.Context
	draftCtx   context.Context
}

func (a *auditingAI) Qualify(ctx context.Context, _ string, _ domain.Channel, _ string) (*domain.Qualification, error) {
	a.qualifyCtx = ctx
	return &domain.Qualification{IdentifiedNeed: "n", EstimatedBudget: "b", Deadline: "d", Score: 5, ScoreReason: "r", RecommendedAction: "a", ProviderUsed: "test"}, nil
}

func (a *auditingAI) DraftReply(ctx context.Context, _ string, _ string) (string, error) {
	a.draftCtx = ctx
	return "draft body", nil
}

func (a *auditingAI) GenerateFollowup(_ context.Context, _ string, _ string, _ int) (string, error) {
	return "", nil
}

func TestQualifyLead_AttachesCallMetaToContext(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:           leadID,
		UserID:       userID,
		Channel:      domain.ChannelEmail,
		ContactName:  "Alice",
		FirstMessage: "hi",
		Status:       domain.StatusNew,
	}
	ai := &auditingAI{}
	uc := NewUseCase(repo, ai, nil)

	_, err := uc.QualifyLead(context.Background(), leadID)
	require.NoError(t, err)

	require.NotNil(t, ai.qualifyCtx, "AIService.Qualify was never called")
	meta, ok := auditdomain.CallMetaFromContext(ai.qualifyCtx)
	require.True(t, ok, "QualifyLead must attach CallMeta to ctx before calling AIService")
	assert.Equal(t, userID, meta.UserID)
	require.NotNil(t, meta.LeadID)
	assert.Equal(t, leadID, *meta.LeadID)
	assert.Equal(t, auditdomain.RequestTypeQualification, meta.RequestType)
	assert.Nil(t, meta.ProspectID, "qualification is not prospect-attributed")
}

func TestRegenerateDraft_AttachesCallMetaToContext(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:           leadID,
		UserID:       userID,
		Channel:      domain.ChannelEmail,
		ContactName:  "Alice",
		FirstMessage: "hi",
		Status:       domain.StatusNew,
	}
	ai := &auditingAI{}
	uc := NewUseCase(repo, ai, nil)

	_, err := uc.RegenerateDraft(context.Background(), leadID)
	require.NoError(t, err)

	require.NotNil(t, ai.draftCtx, "AIService.DraftReply was never called")
	meta, ok := auditdomain.CallMetaFromContext(ai.draftCtx)
	require.True(t, ok, "RegenerateDraft must attach CallMeta to ctx before calling AIService")
	assert.Equal(t, userID, meta.UserID)
	require.NotNil(t, meta.LeadID)
	assert.Equal(t, leadID, *meta.LeadID)
	assert.Equal(t, auditdomain.RequestTypeDraftReply, meta.RequestType)
}
