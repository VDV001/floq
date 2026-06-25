package sequences

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/sequences/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A launch may only target prospects owned by the authenticated caller.
// Passing another user's prospect id (IDOR) is rejected before any message is
// queued — critical now that autopilot can turn a queued message into a real
// send. (#154)
func TestLaunch_ForeignProspect_Rejected(t *testing.T) {
	seqID := uuid.New()
	caller := uuid.New()
	stranger := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{steps: []domain.SequenceStep{
		{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, Channel: domain.StepChannelTelegram, PromptHint: "x"},
	}}
	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID: pid, UserID: stranger, Name: "Victim", Status: "new", VerifyStatus: "valid", IsEligibleForSequence: true,
	}
	uc := NewUseCase(repo, &mockAI{telegramBody: "hi"}, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), caller, seqID, []uuid.UUID{pid}, true)

	require.Error(t, err, "launching on another user's prospect must be rejected")
	assert.Empty(t, repo.messages, "no message is queued for a foreign prospect")
}

// The caller's own prospects launch normally (the ownership guard lets them
// through).
func TestLaunch_OwnProspect_Allowed(t *testing.T) {
	seqID := uuid.New()
	caller := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{steps: []domain.SequenceStep{
		{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, Channel: domain.StepChannelTelegram, PromptHint: "x"},
	}}
	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID: pid, UserID: caller, Name: "Mine", Status: "new", VerifyStatus: "valid", IsEligibleForSequence: true,
	}
	uc := NewUseCase(repo, &mockAI{telegramBody: "hi"}, pr, &mockLeadCreator{})

	require.NoError(t, uc.Launch(context.Background(), caller, seqID, []uuid.UUID{pid}, true))
	assert.Len(t, repo.messages, 1)
}
