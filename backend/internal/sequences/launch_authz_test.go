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

// rollbackTxManager simulates a real DB transaction: writes the inner fn made
// to the repo are discarded if it returns an error (mirrors Postgres ROLLBACK),
// so we can pin that a partial batch doesn't leave the caller's own messages
// queued when a later foreign prospect is rejected.
type rollbackTxManager struct{ repo *mockRepo }

func (m *rollbackTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	snapshot := append([]*domain.OutboundMessage(nil), m.repo.messages...)
	if err := fn(ctx); err != nil {
		m.repo.messages = snapshot
		return err
	}
	return nil
}

// Under a transaction (as in prod), a batch where an earlier prospect is the
// caller's own but a later one is foreign rolls back wholesale — the own
// prospect's already-created messages are discarded, not left half-sent.
func TestLaunch_ForeignInBatch_RollsBackOwnWrites(t *testing.T) {
	seqID := uuid.New()
	caller := uuid.New()
	stranger := uuid.New()
	pidMine := uuid.New()
	pidForeign := uuid.New()

	repo := &mockRepo{steps: []domain.SequenceStep{
		{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, Channel: domain.StepChannelEmail},
	}}
	pr := newMockProspectReader()
	pr.prospects[pidMine] = &domain.ProspectView{ID: pidMine, UserID: caller, Status: "new", VerifyStatus: "valid", IsEligibleForSequence: true}
	pr.prospects[pidForeign] = &domain.ProspectView{ID: pidForeign, UserID: stranger, Status: "new", VerifyStatus: "valid", IsEligibleForSequence: true}
	uc := NewUseCase(repo, &mockAI{coldBody: "hi"}, pr, &mockLeadCreator{}, WithTxManager(&rollbackTxManager{repo: repo}))

	// pidMine processed first (queues a message), then pidForeign is rejected.
	err := uc.Launch(context.Background(), caller, seqID, []uuid.UUID{pidMine, pidForeign})
	require.ErrorIs(t, err, domain.ErrProspectNotOwned)
	assert.Empty(t, repo.messages, "own-prospect writes are rolled back when a foreign prospect aborts the batch")
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
