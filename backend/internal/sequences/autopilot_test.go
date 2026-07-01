package sequences

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/daniil/floq/internal/sequences/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAutopilotChecker is a fake AutopilotChecker recording its call count.
type mockAutopilotChecker struct {
	enabled bool
	delay   time.Duration
	err     error
	calls   int
}

func (m *mockAutopilotChecker) ResolveAutopilot(_ context.Context, _ uuid.UUID) (domain.AutopilotSettings, error) {
	m.calls++
	return domain.AutopilotSettings{Enabled: m.enabled, SendDelay: m.delay}, m.err
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

			_, err := uc.Launch(context.Background(), uuid.Nil, seqID, []uuid.UUID{pid}, true)

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

// When autopilot is on with a send-delay, an auto-approved message is
// scheduled that far in the future — a grace window before the async sender
// (which only picks up approved messages whose scheduled_at <= now) may
// dispatch it, leaving the operator time to intervene.
func TestLaunch_Autopilot_AppliesSendDelay(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()
	repo := &mockRepo{steps: []domain.SequenceStep{
		{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, Channel: domain.StepChannelTelegram, PromptHint: "intro"},
	}}
	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID: pid, UserID: uuid.New(), Name: "Alice", Status: "new", VerifyStatus: "valid", IsEligibleForSequence: true,
	}
	uc := NewUseCase(repo, &mockAI{telegramBody: "hi"}, pr, &mockLeadCreator{},
		WithAutopilotChecker(&mockAutopilotChecker{enabled: true, delay: 5 * time.Minute}))

	before := time.Now().UTC()
	_, errLaunch := uc.Launch(context.Background(), uuid.Nil, seqID, []uuid.UUID{pid}, true)
	require.NoError(t, errLaunch)
	after := time.Now().UTC()

	require.Len(t, repo.messages, 1)
	got := repo.messages[0].ScheduledAt
	assert.False(t, got.Before(before.Add(5*time.Minute)), "scheduled at least the delay into the future")
	assert.False(t, got.After(after.Add(5*time.Minute)), "scheduled at most the delay into the future")
}

// Autopilot auto-approves every message in a multi-prospect, multi-step launch
// (same owner) — not just the first — so the whole batch is dispatched without
// manual approval.
func TestLaunch_Autopilot_ApprovesWholeBatch(t *testing.T) {
	seqID := uuid.New()
	owner := uuid.New()
	pid1 := uuid.New()
	pid2 := uuid.New()
	repo := &mockRepo{steps: []domain.SequenceStep{
		{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, Channel: domain.StepChannelTelegram, PromptHint: "a"},
		{ID: uuid.New(), SequenceID: seqID, StepOrder: 2, Channel: domain.StepChannelTelegram, PromptHint: "b"},
	}}
	pr := newMockProspectReader()
	pr.prospects[pid1] = &domain.ProspectView{ID: pid1, UserID: owner, Name: "A", Status: "new", VerifyStatus: "valid", IsEligibleForSequence: true}
	pr.prospects[pid2] = &domain.ProspectView{ID: pid2, UserID: owner, Name: "B", Status: "new", VerifyStatus: "valid", IsEligibleForSequence: true}
	uc := NewUseCase(repo, &mockAI{telegramBody: "hi"}, pr, &mockLeadCreator{},
		WithAutopilotChecker(&mockAutopilotChecker{enabled: true}))

	_, errLaunch := uc.Launch(context.Background(), uuid.Nil, seqID, []uuid.UUID{pid1, pid2}, true)
	require.NoError(t, errLaunch)

	require.Len(t, repo.messages, 4) // 2 prospects × 2 steps
	for _, m := range repo.messages {
		assert.Equal(t, domain.OutboundStatusApproved, m.Status)
	}
}

// The per-sequence approval gate overrides autopilot: a sequence marked
// RequireApproval keeps its launched messages in Draft (awaiting operator
// approval) even when autopilot is on — closing the gap where autopilot
// auto-sends outbound with no human review. Without the gate, autopilot alone
// decides (its prior behaviour).
func TestLaunch_RequireApproval_OverridesAutopilot(t *testing.T) {
	tests := []struct {
		name            string
		autopilot       bool
		requireApproval bool
		want            domain.OutboundStatus
	}{
		{"autopilot on + gate → draft (gate wins)", true, true, domain.OutboundStatusDraft},
		{"autopilot on + no gate → approved", true, false, domain.OutboundStatusApproved},
		{"autopilot off + gate → draft", false, true, domain.OutboundStatusDraft},
		{"autopilot off + no gate → draft", false, false, domain.OutboundStatusDraft},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seqID := uuid.New()
			owner := uuid.New()
			pid := uuid.New()
			repo := &mockRepo{
				sequences: []domain.Sequence{{ID: seqID, UserID: owner, Name: "S", RequireApproval: tt.requireApproval}},
				steps: []domain.SequenceStep{
					{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, Channel: domain.StepChannelTelegram, PromptHint: "intro"},
				},
			}
			pr := newMockProspectReader()
			pr.prospects[pid] = &domain.ProspectView{
				ID: pid, UserID: owner, Name: "Alice", Status: "new", VerifyStatus: "valid", IsEligibleForSequence: true,
			}
			uc := NewUseCase(repo, &mockAI{telegramBody: "hi"}, pr, &mockLeadCreator{},
				WithAutopilotChecker(&mockAutopilotChecker{enabled: tt.autopilot}))

			_, errLaunch := uc.Launch(context.Background(), owner, seqID, []uuid.UUID{pid}, true)
			require.NoError(t, errLaunch)
			require.Len(t, repo.messages, 1)
			assert.Equal(t, tt.want, repo.messages[0].Status)
		})
	}
}

// A batch that mixes the caller's own prospect with a stranger's is rejected
// whole — no partial send. The ownership guard (every prospect must equal the
// authenticated caller) subsumes the old single-owner rule.
func TestLaunch_BatchWithForeignProspect_RejectedWhole(t *testing.T) {
	seqID := uuid.New()
	caller := uuid.New()
	stranger := uuid.New()
	pidMine := uuid.New()
	pidForeign := uuid.New()
	repo := &mockRepo{
		sequences: []domain.Sequence{{ID: seqID, UserID: caller, Name: "Mine"}},
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, Channel: domain.StepChannelTelegram, PromptHint: "intro"},
		},
	}
	pr := newMockProspectReader()
	pr.prospects[pidMine] = &domain.ProspectView{
		ID: pidMine, UserID: caller, Name: "Mine", Status: "new", VerifyStatus: "valid", IsEligibleForSequence: true,
	}
	pr.prospects[pidForeign] = &domain.ProspectView{
		ID: pidForeign, UserID: stranger, Name: "Foreign", Status: "new", VerifyStatus: "valid", IsEligibleForSequence: true,
	}
	uc := NewUseCase(repo, &mockAI{telegramBody: "hi"}, pr, &mockLeadCreator{},
		WithAutopilotChecker(&mockAutopilotChecker{enabled: true}))

	_, err := uc.Launch(context.Background(), caller, seqID, []uuid.UUID{pidMine, pidForeign}, true)
	require.ErrorIs(t, err, domain.ErrProspectNotOwned)
	// No message is queued for the foreign prospect.
	for _, m := range repo.messages {
		assert.Equal(t, pidMine, m.ProspectID, "only the caller's own prospect may be queued")
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

	_, errLaunch := uc.Launch(context.Background(), uuid.Nil, seqID, []uuid.UUID{pid}, true)
	require.NoError(t, errLaunch)
	require.Len(t, repo.messages, 1)
	assert.Equal(t, domain.OutboundStatusDraft, repo.messages[0].Status)
}
