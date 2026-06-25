package sequences

import (
	"context"
	"errors"
	"testing"

	"github.com/daniil/floq/internal/sequences/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAutopilotChecker is a fake AutopilotChecker recording its call count.
type mockAutopilotChecker struct {
	enabled bool
	err     error
	calls   int
}

func (m *mockAutopilotChecker) IsAutopilotEnabled(_ context.Context, _ uuid.UUID) (bool, error) {
	m.calls++
	return m.enabled, m.err
}

// When autopilot is enabled, launch promotes each queued message straight to
// Approved so the async sender dispatches it without a manual approval step.
// When disabled — the default — messages stay Draft and wait for a human.
// A checker error fails the launch: the send mode is never guessed, so an
// unreadable setting can never silently auto-send.
func TestLaunch_Autopilot(t *testing.T) {
	checkerErr := errors.New("settings store down")

	tests := []struct {
		name       string
		enabled    bool
		checkerErr error
		wantErr    bool
		wantStatus domain.OutboundStatus
		wantMsgs   int
	}{
		{"autopilot on → message approved", true, nil, false, domain.OutboundStatusApproved, 1},
		{"autopilot off → message stays draft", false, nil, false, domain.OutboundStatusDraft, 1},
		{"checker error → launch fails, nothing queued", false, checkerErr, true, "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seqID := uuid.New()
			pid := uuid.New()
			repo := &mockRepo{steps: []domain.SequenceStep{
				{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, Channel: domain.StepChannelTelegram, PromptHint: "intro"},
			}}
			pr := newMockProspectReader()
			pr.prospects[pid] = &domain.ProspectView{
				ID: pid, UserID: uuid.New(), Name: "Alice", Company: "Acme", Title: "CEO",
				Status: "new", VerifyStatus: "valid", IsEligibleForSequence: true,
			}
			checker := &mockAutopilotChecker{enabled: tt.enabled, err: tt.checkerErr}
			uc := NewUseCase(repo, &mockAI{telegramBody: "hi"}, pr, &mockLeadCreator{},
				WithAutopilotChecker(checker))

			err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid}, true)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, 1, checker.calls, "autopilot must be checked exactly once per launch")
			require.Len(t, repo.messages, tt.wantMsgs)
			if tt.wantMsgs > 0 {
				assert.Equal(t, tt.wantStatus, repo.messages[0].Status)
			}
		})
	}
}

// A launch batch must belong to a single owner. Autopilot (and the email
// preflight) resolve their decision once from the first prospect's owner, so a
// mixed-owner batch would otherwise apply one owner's settings to another
// owner's messages — e.g. auto-approving owner B's message because owner A has
// autopilot on. Such a batch is rejected before any wrong-owner message is
// created.
func TestLaunch_MixedOwners_Rejected(t *testing.T) {
	seqID := uuid.New()
	pidA := uuid.New()
	pidB := uuid.New()
	repo := &mockRepo{steps: []domain.SequenceStep{
		{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, Channel: domain.StepChannelTelegram, PromptHint: "intro"},
	}}
	pr := newMockProspectReader()
	pr.prospects[pidA] = &domain.ProspectView{
		ID: pidA, UserID: uuid.New(), Name: "A", Status: "new", VerifyStatus: "valid", IsEligibleForSequence: true,
	}
	pr.prospects[pidB] = &domain.ProspectView{
		ID: pidB, UserID: uuid.New(), Name: "B", Status: "new", VerifyStatus: "valid", IsEligibleForSequence: true,
	}
	// Owner A has autopilot on; without the single-owner guard, owner B's
	// message would be auto-approved using A's setting.
	uc := NewUseCase(repo, &mockAI{telegramBody: "hi"}, pr, &mockLeadCreator{},
		WithAutopilotChecker(&mockAutopilotChecker{enabled: true}))

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pidA, pidB}, true)
	require.Error(t, err)
	// Owner B never gets an auto-approved message from owner A's setting.
	for _, m := range repo.messages {
		assert.Equal(t, pidA, m.ProspectID, "only the first owner's message may exist")
	}
}

// A nil checker (the default wiring before this feature) leaves launch
// behaviour unchanged: messages are created as drafts and wait for manual
// approval — autopilot is off.
func TestLaunch_NoAutopilotChecker_StaysDraft(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()
	repo := &mockRepo{steps: []domain.SequenceStep{
		{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, Channel: domain.StepChannelTelegram, PromptHint: "intro"},
	}}
	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID: pid, UserID: uuid.New(), Name: "Alice", Status: "new", VerifyStatus: "valid", IsEligibleForSequence: true,
	}
	uc := NewUseCase(repo, &mockAI{telegramBody: "hi"}, pr, &mockLeadCreator{})

	require.NoError(t, uc.Launch(context.Background(), seqID, []uuid.UUID{pid}, true))
	require.Len(t, repo.messages, 1)
	assert.Equal(t, domain.OutboundStatusDraft, repo.messages[0].Status)
}
