package sequences

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/daniil/floq/internal/sequences/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock Repository ---

type mockRepo struct {
	steps    []domain.SequenceStep
	messages []*domain.OutboundMessage
	stepErr  error
	msgErr   error
}

func (m *mockRepo) ListSequences(_ context.Context, _ uuid.UUID) ([]domain.Sequence, error) {
	return nil, nil
}
func (m *mockRepo) GetSequence(_ context.Context, _ uuid.UUID) (*domain.Sequence, error) {
	return nil, nil
}
func (m *mockRepo) CreateSequence(_ context.Context, _ *domain.Sequence) error { return nil }
func (m *mockRepo) UpdateSequence(_ context.Context, _ *domain.Sequence) error { return nil }
func (m *mockRepo) DeleteSequence(_ context.Context, _ uuid.UUID) error       { return nil }
func (m *mockRepo) ToggleActive(_ context.Context, _ uuid.UUID, _ bool) error { return nil }
func (m *mockRepo) ListSteps(_ context.Context, _ uuid.UUID) ([]domain.SequenceStep, error) {
	return m.steps, m.stepErr
}
func (m *mockRepo) CreateStep(_ context.Context, _ *domain.SequenceStep) error { return nil }
func (m *mockRepo) CreateOutboundMessage(_ context.Context, msg *domain.OutboundMessage) error {
	if m.msgErr != nil {
		return m.msgErr
	}
	m.messages = append(m.messages, msg)
	return nil
}
func (m *mockRepo) ListOutboundQueue(_ context.Context, _ uuid.UUID) ([]domain.OutboundMessage, error) {
	return nil, nil
}
func (m *mockRepo) DeleteStep(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (m *mockRepo) ListSentMessages(_ context.Context, _ uuid.UUID) ([]domain.OutboundMessage, error) {
	return nil, nil
}
func (m *mockRepo) UpdateOutboundStatus(_ context.Context, _ uuid.UUID, _ domain.OutboundStatus) error {
	return nil
}
func (m *mockRepo) UpdateOutboundBody(_ context.Context, _ uuid.UUID, _ string) error { return nil }
func (m *mockRepo) GetPendingSends(_ context.Context) ([]domain.OutboundMessage, error) {
	return nil, nil
}
func (m *mockRepo) MarkSent(_ context.Context, _ uuid.UUID) error    { return nil }
func (m *mockRepo) MarkBounced(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockRepo) GetStats(_ context.Context, _ uuid.UUID) (*domain.Stats, error) {
	return nil, nil
}

// --- Mock AI Generator ---

type mockAI struct {
	coldBody     string
	telegramBody string
	callBody     string
	err          error
	calls        int
}

func (m *mockAI) GenerateColdMessage(_ context.Context, _, _, _, _, _, _, _ string) (string, error) {
	m.calls++
	return m.coldBody, m.err
}

func (m *mockAI) GenerateTelegramMessage(_ context.Context, _, _, _, _, _, _, _ string) (string, error) {
	m.calls++
	return m.telegramBody, m.err
}

func (m *mockAI) GenerateCallBrief(_ context.Context, _, _, _, _, _, _ string) (string, error) {
	m.calls++
	return m.callBody, m.err
}

// --- Mock Prospect Reader ---

type mockProspectReader struct {
	prospects     map[uuid.UUID]*domain.ProspectView
	statusUpdates map[uuid.UUID]string
	updateErr     error
}

func newMockProspectReader() *mockProspectReader {
	return &mockProspectReader{
		prospects:     make(map[uuid.UUID]*domain.ProspectView),
		statusUpdates: make(map[uuid.UUID]string),
	}
}

func (m *mockProspectReader) GetProspect(_ context.Context, id uuid.UUID) (*domain.ProspectView, error) {
	p, ok := m.prospects[id]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (m *mockProspectReader) UpdateStatus(_ context.Context, id uuid.UUID, status string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.statusUpdates[id] = status
	return nil
}

// --- Mock Lead Creator ---

type mockLeadCreator struct {
	createdFrom []uuid.UUID
	returnID    uuid.UUID
	err         error
}

func (m *mockLeadCreator) CreateLeadFromProspect(_ context.Context, prospect *domain.ProspectView, _ uuid.UUID) (uuid.UUID, error) {
	m.createdFrom = append(m.createdFrom, prospect.ID)
	return m.returnID, m.err
}

// --- Launch Tests ---

func TestLaunch_HappyPath(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail, PromptHint: "intro"},
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 2, DelayDays: 3, Channel: domain.StepChannelTelegram, PromptHint: "follow up"},
		},
	}

	ai := &mockAI{coldBody: "Hello email", telegramBody: "Hello tg"}

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID:           pid,
		UserID:       uuid.New(),
		Name:         "Alice",
		Company:      "Acme",
		Title:        "CEO",
		Email:        "alice@acme.com",
		Status:       "new",
		VerifyStatus: "valid",
	}

	uc := NewUseCase(repo, ai, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid})
	require.NoError(t, err)

	// Should create 2 outbound messages (one per step)
	require.Len(t, repo.messages, 2)

	assert.Equal(t, domain.StepChannelEmail, repo.messages[0].Channel)
	assert.Equal(t, "Hello email", repo.messages[0].Body)
	assert.Equal(t, domain.OutboundStatusDraft, repo.messages[0].Status)
	assert.Equal(t, 1, repo.messages[0].StepOrder)

	assert.Equal(t, domain.StepChannelTelegram, repo.messages[1].Channel)
	assert.Equal(t, "Hello tg", repo.messages[1].Body)
	assert.Equal(t, 2, repo.messages[1].StepOrder)

	// ScheduledAt: step2 should be 3 days after step1
	assert.True(t, repo.messages[1].ScheduledAt.After(repo.messages[0].ScheduledAt))

	// Prospect should be marked as in_sequence
	assert.Equal(t, "in_sequence", pr.statusUpdates[pid])

	// AI should have been called twice
	assert.Equal(t, 2, ai.calls)
}

func TestLaunch_SkipConverted(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail},
		},
	}

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID:           pid,
		Status:       "converted",
		VerifyStatus: "valid",
	}

	uc := NewUseCase(repo, &mockAI{coldBody: "hi"}, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid})
	require.NoError(t, err)

	// No messages should be created for converted prospects
	assert.Empty(t, repo.messages)
	assert.Empty(t, pr.statusUpdates)
}

func TestLaunch_SkipOptedOut(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail},
		},
	}

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID:           pid,
		Status:       "opted_out",
		VerifyStatus: "valid",
	}

	uc := NewUseCase(repo, &mockAI{coldBody: "hi"}, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid})
	require.NoError(t, err)

	assert.Empty(t, repo.messages)
}

func TestLaunch_SkipInSequence(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail},
		},
	}

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID:           pid,
		Status:       "in_sequence",
		VerifyStatus: "valid",
	}

	uc := NewUseCase(repo, &mockAI{coldBody: "hi"}, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid})
	require.NoError(t, err)

	assert.Empty(t, repo.messages)
}

func TestLaunch_SkipInvalidVerify(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail},
		},
	}

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID:           pid,
		Status:       "new",
		VerifyStatus: "invalid",
	}

	uc := NewUseCase(repo, &mockAI{coldBody: "hi"}, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid})
	require.NoError(t, err)

	assert.Empty(t, repo.messages)
}

func TestLaunch_SkipNotCheckedWithEmail(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail},
		},
	}

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID:           pid,
		Status:       "new",
		Email:        "alice@acme.com",
		VerifyStatus: "not_checked",
	}

	uc := NewUseCase(repo, &mockAI{coldBody: "hi"}, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid})
	require.NoError(t, err)

	assert.Empty(t, repo.messages)
}

func TestLaunch_AllowNotCheckedWithoutEmail(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail},
		},
	}

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID:           pid,
		UserID:       uuid.New(),
		Name:         "Bob",
		Status:       "new",
		Email:        "", // no email
		VerifyStatus: "not_checked",
	}

	ai := &mockAI{coldBody: "Hey Bob"}
	uc := NewUseCase(repo, ai, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid})
	require.NoError(t, err)

	// Should proceed: not_checked + no email is allowed
	require.Len(t, repo.messages, 1)
	assert.Equal(t, "Hey Bob", repo.messages[0].Body)
}

func TestLaunch_NoSteps(t *testing.T) {
	repo := &mockRepo{steps: nil}
	pr := newMockProspectReader()
	uc := NewUseCase(repo, &mockAI{}, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), uuid.New(), []uuid.UUID{uuid.New()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sequence has no steps")
}

func TestLaunch_ListStepsError(t *testing.T) {
	repo := &mockRepo{stepErr: errors.New("db down")}
	pr := newMockProspectReader()
	uc := NewUseCase(repo, &mockAI{}, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), uuid.New(), []uuid.UUID{uuid.New()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list steps")
}

func TestLaunch_ProspectNotFound(t *testing.T) {
	seqID := uuid.New()
	missingID := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail},
		},
	}

	pr := newMockProspectReader() // empty -- no prospects
	uc := NewUseCase(repo, &mockAI{coldBody: "hi"}, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{missingID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLaunch_AIGeneratorError(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail},
		},
	}

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID:           pid,
		Status:       "new",
		VerifyStatus: "valid",
	}

	ai := &mockAI{err: errors.New("openai timeout")}
	uc := NewUseCase(repo, ai, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generate message")
}

func TestLaunch_PhoneCallChannel(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelPhoneCall},
		},
	}

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID:           pid,
		UserID:       uuid.New(),
		Name:         "Charlie",
		Status:       "new",
		VerifyStatus: "valid",
	}

	ai := &mockAI{callBody: "Call brief for Charlie"}
	uc := NewUseCase(repo, ai, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid})
	require.NoError(t, err)

	require.Len(t, repo.messages, 1)
	assert.Equal(t, domain.StepChannelPhoneCall, repo.messages[0].Channel)
	assert.Equal(t, "Call brief for Charlie", repo.messages[0].Body)
}

func TestLaunch_MultipleProspects_MixedStatuses(t *testing.T) {
	seqID := uuid.New()
	pidOK := uuid.New()
	pidConverted := uuid.New()
	pidInvalid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail},
		},
	}

	pr := newMockProspectReader()
	pr.prospects[pidOK] = &domain.ProspectView{
		ID: pidOK, Name: "Good", Status: "new", VerifyStatus: "valid",
	}
	pr.prospects[pidConverted] = &domain.ProspectView{
		ID: pidConverted, Name: "Conv", Status: "converted", VerifyStatus: "valid",
	}
	pr.prospects[pidInvalid] = &domain.ProspectView{
		ID: pidInvalid, Name: "Bad", Status: "new", VerifyStatus: "invalid",
	}

	ai := &mockAI{coldBody: "Hello"}
	uc := NewUseCase(repo, ai, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pidOK, pidConverted, pidInvalid})
	require.NoError(t, err)

	// Only pidOK should get a message
	require.Len(t, repo.messages, 1)
	assert.Equal(t, pidOK, repo.messages[0].ProspectID)
	assert.Equal(t, "in_sequence", pr.statusUpdates[pidOK])

	// Skipped prospects should not have status updates
	_, hasConverted := pr.statusUpdates[pidConverted]
	_, hasInvalid := pr.statusUpdates[pidInvalid]
	assert.False(t, hasConverted)
	assert.False(t, hasInvalid)
}

func TestLaunch_CumulativeDelay(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 1, Channel: domain.StepChannelEmail},
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 2, DelayDays: 2, Channel: domain.StepChannelEmail},
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 3, DelayDays: 4, Channel: domain.StepChannelEmail},
		},
	}

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID: pid, Name: "Alice", Status: "new", VerifyStatus: "valid",
	}

	ai := &mockAI{coldBody: "msg"}
	uc := NewUseCase(repo, ai, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid})
	require.NoError(t, err)
	require.Len(t, repo.messages, 3)

	// Delays are cumulative: 1, 1+2=3, 1+2+4=7
	day0 := repo.messages[0].ScheduledAt
	diff1 := repo.messages[1].ScheduledAt.Sub(day0).Hours() / 24
	diff2 := repo.messages[2].ScheduledAt.Sub(day0).Hours() / 24

	assert.InDelta(t, 2.0, diff1, 0.01) // step2 is 2 more days after step1
	assert.InDelta(t, 6.0, diff2, 0.01) // step3 is 6 more days after step1 (2+4)
}

// --- ConvertToLead Tests ---

func TestConvertToLead_HappyPath(t *testing.T) {
	pid := uuid.New()
	userID := uuid.New()
	leadID := uuid.New()

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID:     pid,
		UserID: userID,
		Name:   "Alice",
		Status: "in_sequence",
	}

	lc := &mockLeadCreator{returnID: leadID}
	uc := NewUseCase(&mockRepo{}, &mockAI{}, pr, lc)

	err := uc.ConvertToLead(context.Background(), pid)
	require.NoError(t, err)

	// Lead should have been created from prospect
	require.Len(t, lc.createdFrom, 1)
	assert.Equal(t, pid, lc.createdFrom[0])

	// Prospect should be marked as converted
	assert.Equal(t, "converted", pr.statusUpdates[pid])
}

func TestConvertToLead_ProspectNotFound(t *testing.T) {
	pr := newMockProspectReader()
	uc := NewUseCase(&mockRepo{}, &mockAI{}, pr, &mockLeadCreator{})

	err := uc.ConvertToLead(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prospect not found")
}

func TestConvertToLead_LeadCreatorError(t *testing.T) {
	pid := uuid.New()

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID:     pid,
		UserID: uuid.New(),
		Name:   "Alice",
	}

	lc := &mockLeadCreator{err: fmt.Errorf("db error")}
	uc := NewUseCase(&mockRepo{}, &mockAI{}, pr, lc)

	err := uc.ConvertToLead(context.Background(), pid)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create lead")
}

func TestConvertToLead_UpdateStatusError(t *testing.T) {
	pid := uuid.New()

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID:     pid,
		UserID: uuid.New(),
		Name:   "Alice",
	}
	pr.updateErr = errors.New("status update failed")

	lc := &mockLeadCreator{returnID: uuid.New()}
	uc := NewUseCase(&mockRepo{}, &mockAI{}, pr, lc)

	err := uc.ConvertToLead(context.Background(), pid)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update prospect")
}
