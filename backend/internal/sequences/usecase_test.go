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
	sequences []domain.Sequence
	steps     []domain.SequenceStep
	messages  []*domain.OutboundMessage
	stepErr   error
	msgErr    error
	seqErr    error
	statsVal  *domain.Stats
	statsErr  error
	queue     []domain.OutboundMessage
	queueErr  error
	sent      []domain.OutboundMessage
	sentErr   error

	statusUpdateErr error
	bodyUpdateErr   error
	deleteSeqErr    error
	deleteStepErr   error
	toggleErr       error
	updateSeqErr    error
	createSeqErr    error
	createStepErr   error
	openedErr       error

	feedbackSaved []promptFeedbackRecord
	feedback      []domain.PromptFeedback
	history       []domain.ConversationEntry
}

type promptFeedbackRecord struct {
	userID, original, edited, prospectCtx, channel string
}

func (m *mockRepo) ListSequences(_ context.Context, _ uuid.UUID) ([]domain.Sequence, error) {
	return m.sequences, m.seqErr
}
func (m *mockRepo) GetSequence(_ context.Context, id uuid.UUID) (*domain.Sequence, error) {
	if m.seqErr != nil {
		return nil, m.seqErr
	}
	for i := range m.sequences {
		if m.sequences[i].ID == id {
			return &m.sequences[i], nil
		}
	}
	return nil, nil
}
func (m *mockRepo) CreateSequence(_ context.Context, _ *domain.Sequence) error {
	return m.createSeqErr
}
func (m *mockRepo) UpdateSequence(_ context.Context, _ *domain.Sequence) error {
	return m.updateSeqErr
}
func (m *mockRepo) DeleteSequence(_ context.Context, _ uuid.UUID) error { return m.deleteSeqErr }
func (m *mockRepo) ToggleActive(_ context.Context, _ uuid.UUID, _ bool) error {
	return m.toggleErr
}
func (m *mockRepo) ListSteps(_ context.Context, _ uuid.UUID) ([]domain.SequenceStep, error) {
	return m.steps, m.stepErr
}
func (m *mockRepo) CreateStep(_ context.Context, _ *domain.SequenceStep) error {
	return m.createStepErr
}
func (m *mockRepo) CreateOutboundMessage(_ context.Context, msg *domain.OutboundMessage) error {
	if m.msgErr != nil {
		return m.msgErr
	}
	m.messages = append(m.messages, msg)
	return nil
}
func (m *mockRepo) ListOutboundQueue(_ context.Context, _ uuid.UUID) ([]domain.OutboundMessage, error) {
	return m.queue, m.queueErr
}
func (m *mockRepo) DeleteStep(_ context.Context, _ uuid.UUID) error {
	return m.deleteStepErr
}
func (m *mockRepo) ListSentMessages(_ context.Context, _ uuid.UUID) ([]domain.OutboundMessage, error) {
	return m.sent, m.sentErr
}
func (m *mockRepo) UpdateOutboundStatus(_ context.Context, _ uuid.UUID, _ domain.OutboundStatus) error {
	return m.statusUpdateErr
}
func (m *mockRepo) UpdateOutboundBody(_ context.Context, _ uuid.UUID, _ string) error {
	return m.bodyUpdateErr
}
func (m *mockRepo) GetPendingSends(_ context.Context) ([]domain.OutboundMessage, error) {
	return nil, nil
}
func (m *mockRepo) MarkSent(_ context.Context, _ uuid.UUID) error    { return nil }
func (m *mockRepo) MarkBounced(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockRepo) MarkOpened(_ context.Context, _ uuid.UUID) error  { return m.openedErr }
func (m *mockRepo) GetStats(_ context.Context, _ uuid.UUID) (*domain.Stats, error) {
	return m.statsVal, m.statsErr
}
func (m *mockRepo) GetOutboundMessage(_ context.Context, id uuid.UUID) (*domain.OutboundMessage, error) {
	for _, msg := range m.messages {
		if msg.ID == id {
			return msg, nil
		}
	}
	return nil, nil
}
func (m *mockRepo) SavePromptFeedback(_ context.Context, userID uuid.UUID, original, edited, prospectCtx, channel string) error {
	m.feedbackSaved = append(m.feedbackSaved, promptFeedbackRecord{
		userID: userID.String(), original: original, edited: edited, prospectCtx: prospectCtx, channel: channel,
	})
	return nil
}
func (m *mockRepo) GetRecentFeedback(_ context.Context, _ uuid.UUID, _ int) ([]domain.PromptFeedback, error) {
	return m.feedback, nil
}
func (m *mockRepo) GetConversationHistory(_ context.Context, _ uuid.UUID) ([]domain.ConversationEntry, error) {
	return m.history, nil
}

// --- Mock AI Generator ---

type mockAI struct {
	coldBody     string
	telegramBody string
	callBody     string
	err          error
	calls        int
}

func (m *mockAI) GenerateColdMessage(_ context.Context, _, _, _, _, _, _, _, _ string) (string, error) {
	m.calls++
	return m.coldBody, m.err
}

func (m *mockAI) GenerateTelegramMessage(_ context.Context, _, _, _, _, _, _, _, _ string) (string, error) {
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

// --- ListSequences Tests ---

func TestListSequences(t *testing.T) {
	userID := uuid.New()
	seqs := []domain.Sequence{
		{ID: uuid.New(), UserID: userID, Name: "Seq1"},
		{ID: uuid.New(), UserID: userID, Name: "Seq2"},
	}
	repo := &mockRepo{sequences: seqs}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	result, err := uc.ListSequences(context.Background(), userID)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "Seq1", result[0].Name)
}

func TestListSequences_Error(t *testing.T) {
	repo := &mockRepo{seqErr: errors.New("db error")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	_, err := uc.ListSequences(context.Background(), uuid.New())
	require.Error(t, err)
}

// --- GetSequence Tests ---

func TestGetSequence(t *testing.T) {
	id := uuid.New()
	repo := &mockRepo{sequences: []domain.Sequence{{ID: id, Name: "Test"}}}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	result, err := uc.GetSequence(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Test", result.Name)
}

func TestGetSequence_NotFound(t *testing.T) {
	repo := &mockRepo{}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	result, err := uc.GetSequence(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, result)
}

// --- CreateSequence Tests ---

func TestCreateSequence(t *testing.T) {
	repo := &mockRepo{}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	s := domain.NewSequence(uuid.New(), "New Seq")
	err := uc.CreateSequence(context.Background(), s)
	require.NoError(t, err)
}

func TestCreateSequence_Error(t *testing.T) {
	repo := &mockRepo{createSeqErr: errors.New("create failed")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	s := domain.NewSequence(uuid.New(), "New Seq")
	err := uc.CreateSequence(context.Background(), s)
	require.Error(t, err)
}

// --- DeleteSequence Tests ---

func TestDeleteSequence(t *testing.T) {
	repo := &mockRepo{}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.DeleteSequence(context.Background(), uuid.New())
	require.NoError(t, err)
}

func TestDeleteSequence_Error(t *testing.T) {
	repo := &mockRepo{deleteSeqErr: errors.New("delete failed")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.DeleteSequence(context.Background(), uuid.New())
	require.Error(t, err)
}

// --- AddStep / DeleteStep Tests ---

func TestCreateStep(t *testing.T) {
	repo := &mockRepo{}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	step := domain.NewSequenceStep(uuid.New(), 1, 0, domain.StepChannelEmail, "intro")
	err := uc.CreateStep(context.Background(), step)
	require.NoError(t, err)
}

func TestDeleteStep(t *testing.T) {
	repo := &mockRepo{}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.DeleteStep(context.Background(), uuid.New())
	require.NoError(t, err)
}

func TestDeleteStep_Error(t *testing.T) {
	repo := &mockRepo{deleteStepErr: errors.New("step not found")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.DeleteStep(context.Background(), uuid.New())
	require.Error(t, err)
}

// --- ApproveMessage / RejectMessage Tests ---

func TestApproveMessage(t *testing.T) {
	repo := &mockRepo{}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.ApproveMessage(context.Background(), uuid.New())
	require.NoError(t, err)
}

func TestApproveMessage_Error(t *testing.T) {
	repo := &mockRepo{statusUpdateErr: errors.New("update failed")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.ApproveMessage(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestRejectMessage(t *testing.T) {
	repo := &mockRepo{}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.RejectMessage(context.Background(), uuid.New())
	require.NoError(t, err)
}

func TestRejectMessage_Error(t *testing.T) {
	repo := &mockRepo{statusUpdateErr: errors.New("update failed")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.RejectMessage(context.Background(), uuid.New())
	require.Error(t, err)
}

// --- EditMessage Tests ---

func TestEditMessage_HappyPath(t *testing.T) {
	msgID := uuid.New()
	pid := uuid.New()
	userID := uuid.New()

	repo := &mockRepo{
		messages: []*domain.OutboundMessage{
			{ID: msgID, ProspectID: pid, Body: "original body", Channel: domain.StepChannelEmail},
		},
	}
	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{ID: pid, UserID: userID, Context: "CEO at Acme"}

	uc := NewUseCase(repo, &mockAI{}, pr, &mockLeadCreator{})
	err := uc.EditMessage(context.Background(), msgID, "edited body")
	require.NoError(t, err)

	// Should have saved feedback
	require.Len(t, repo.feedbackSaved, 1)
	assert.Equal(t, "original body", repo.feedbackSaved[0].original)
	assert.Equal(t, "edited body", repo.feedbackSaved[0].edited)
}

func TestEditMessage_NotFound(t *testing.T) {
	repo := &mockRepo{}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.EditMessage(context.Background(), uuid.New(), "new body")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "message not found")
}

func TestEditMessage_SameBody_NoFeedback(t *testing.T) {
	msgID := uuid.New()
	body := "same body"

	repo := &mockRepo{
		messages: []*domain.OutboundMessage{
			{ID: msgID, Body: body, Channel: domain.StepChannelEmail},
		},
	}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.EditMessage(context.Background(), msgID, body)
	require.NoError(t, err)
	assert.Empty(t, repo.feedbackSaved)
}

// --- GetQueue / GetSent / GetStats Tests ---

func TestGetQueue(t *testing.T) {
	msgs := []domain.OutboundMessage{{ID: uuid.New(), Body: "msg1"}}
	repo := &mockRepo{queue: msgs}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	result, err := uc.GetQueue(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestGetQueue_Error(t *testing.T) {
	repo := &mockRepo{queueErr: errors.New("db error")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	_, err := uc.GetQueue(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestGetSent(t *testing.T) {
	msgs := []domain.OutboundMessage{{ID: uuid.New(), Body: "sent1"}}
	repo := &mockRepo{sent: msgs}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	result, err := uc.GetSent(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestGetStats(t *testing.T) {
	stats := &domain.Stats{Draft: 5, Approved: 3, Sent: 10, Opened: 2, Replied: 1, Bounced: 0}
	repo := &mockRepo{statsVal: stats}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	result, err := uc.GetStats(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 5, result.Draft)
	assert.Equal(t, 10, result.Sent)
}

func TestGetStats_Error(t *testing.T) {
	repo := &mockRepo{statsErr: errors.New("db error")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	_, err := uc.GetStats(context.Background(), uuid.New())
	require.Error(t, err)
}

// --- ToggleActive / UpdateSequence Tests ---

func TestToggleActive(t *testing.T) {
	repo := &mockRepo{}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.ToggleActive(context.Background(), uuid.New(), true)
	require.NoError(t, err)
}

func TestToggleActive_Error(t *testing.T) {
	repo := &mockRepo{toggleErr: errors.New("toggle failed")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.ToggleActive(context.Background(), uuid.New(), true)
	require.Error(t, err)
}

func TestUpdateSequence(t *testing.T) {
	repo := &mockRepo{}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.UpdateSequence(context.Background(), &domain.Sequence{ID: uuid.New(), Name: "Updated"})
	require.NoError(t, err)
}

func TestUpdateSequence_Error(t *testing.T) {
	repo := &mockRepo{updateSeqErr: errors.New("update failed")}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.UpdateSequence(context.Background(), &domain.Sequence{ID: uuid.New(), Name: "Updated"})
	require.Error(t, err)
}

// --- MarkOpened Tests ---

func TestMarkOpened(t *testing.T) {
	repo := &mockRepo{}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.MarkOpened(context.Background(), uuid.New())
	require.NoError(t, err)
}

// --- Launch with SendNow ---

func TestLaunch_SendNow(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail},
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 2, DelayDays: 5, Channel: domain.StepChannelEmail},
		},
	}

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID: pid, UserID: uuid.New(), Name: "Alice", Status: "new", VerifyStatus: "valid",
	}

	ai := &mockAI{coldBody: "msg"}
	uc := NewUseCase(repo, ai, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid}, true)
	require.NoError(t, err)
	require.Len(t, repo.messages, 2)

	// With sendNow=true, both messages should have the same scheduledAt (immediate)
	assert.Equal(t, repo.messages[0].ScheduledAt, repo.messages[1].ScheduledAt)
}

// --- Launch with conversation history ---

func TestLaunch_WithConversationHistory(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail},
		},
		history: []domain.ConversationEntry{
			{Body: "Previous message 1"},
			{Body: "Previous message 2"},
		},
	}

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID: pid, UserID: uuid.New(), Name: "Alice", Status: "new", VerifyStatus: "valid",
	}

	ai := &mockAI{coldBody: "follow up"}
	uc := NewUseCase(repo, ai, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid})
	require.NoError(t, err)
	require.Len(t, repo.messages, 1)
}

// --- Launch with feedback examples ---

func TestLaunch_WithFeedbackExamples(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail},
		},
		feedback: []domain.PromptFeedback{
			{OriginalBody: "Was this", EditedBody: "Now this"},
		},
	}

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID: pid, UserID: uuid.New(), Name: "Alice", Status: "new", VerifyStatus: "valid",
	}

	ai := &mockAI{coldBody: "msg"}
	uc := NewUseCase(repo, ai, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid})
	require.NoError(t, err)
	require.Len(t, repo.messages, 1)
}

// --- Launch with TxManager ---

type mockTxManager struct {
	called bool
	err    error
}

func (m *mockTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	m.called = true
	if m.err != nil {
		return m.err
	}
	return fn(ctx)
}

func TestLaunch_WithTxManager(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail},
		},
	}

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID: pid, UserID: uuid.New(), Name: "Alice", Status: "new", VerifyStatus: "valid",
	}

	txMgr := &mockTxManager{}
	ai := &mockAI{coldBody: "msg"}
	uc := NewUseCase(repo, ai, pr, &mockLeadCreator{}, WithTxManager(txMgr))

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid})
	require.NoError(t, err)
	assert.True(t, txMgr.called)
}

// --- buildFeedbackExamples Tests ---

func TestBuildFeedbackExamples_Empty(t *testing.T) {
	repo := &mockRepo{}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	result := uc.buildFeedbackExamples(context.Background(), uuid.New())
	assert.Empty(t, result)
}

func TestBuildFeedbackExamples_WithFeedback(t *testing.T) {
	repo := &mockRepo{
		feedback: []domain.PromptFeedback{
			{OriginalBody: "Original", EditedBody: "Edited"},
		},
	}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	result := uc.buildFeedbackExamples(context.Background(), uuid.New())
	assert.Contains(t, result, "Original")
	assert.Contains(t, result, "Edited")
	assert.Contains(t, result, "Примеры правок менеджера")
}

// --- NewUseCase Tests ---

func TestEditMessage_BodyUpdateError(t *testing.T) {
	msgID := uuid.New()
	repo := &mockRepo{
		messages:      []*domain.OutboundMessage{{ID: msgID, Body: "old", Channel: domain.StepChannelEmail}},
		bodyUpdateErr: errors.New("update body failed"),
	}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.EditMessage(context.Background(), msgID, "new")
	require.Error(t, err)
}

func TestEditMessage_GetOriginalError(t *testing.T) {
	// mockRepo returns nil for unknown IDs -> "message not found"
	repo := &mockRepo{}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.EditMessage(context.Background(), uuid.New(), "new")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "message not found")
}

func TestLaunch_CreateOutboundMessageError(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail},
		},
		msgErr: errors.New("insert failed"),
	}

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID: pid, UserID: uuid.New(), Name: "Alice", Status: "new", VerifyStatus: "valid",
	}

	uc := NewUseCase(repo, &mockAI{coldBody: "msg"}, pr, &mockLeadCreator{})
	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create outbound message")
}

func TestLaunch_UpdateProspectStatusError(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail},
		},
	}

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID: pid, UserID: uuid.New(), Name: "Alice", Status: "new", VerifyStatus: "valid",
	}
	pr.updateErr = errors.New("status update failed")

	uc := NewUseCase(repo, &mockAI{coldBody: "msg"}, pr, &mockLeadCreator{})
	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update prospect status")
}

func TestLaunch_TelegramAIError(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelTelegram},
		},
	}

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID: pid, UserID: uuid.New(), Status: "new", VerifyStatus: "valid",
	}

	ai := &mockAI{err: errors.New("telegram gen error")}
	uc := NewUseCase(repo, ai, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generate message")
}

func TestLaunch_PhoneCallAIError(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelPhoneCall},
		},
	}

	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{
		ID: pid, UserID: uuid.New(), Status: "new", VerifyStatus: "valid",
	}

	ai := &mockAI{err: errors.New("call gen error")}
	uc := NewUseCase(repo, ai, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generate message")
}

func TestEditMessage_ChannelEmpty_DefaultsToEmail(t *testing.T) {
	msgID := uuid.New()
	pid := uuid.New()
	userID := uuid.New()

	repo := &mockRepo{
		messages: []*domain.OutboundMessage{
			{ID: msgID, ProspectID: pid, Body: "original", Channel: ""}, // empty channel
		},
	}
	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{ID: pid, UserID: userID}

	uc := NewUseCase(repo, &mockAI{}, pr, &mockLeadCreator{})
	err := uc.EditMessage(context.Background(), msgID, "edited")
	require.NoError(t, err)

	require.Len(t, repo.feedbackSaved, 1)
	assert.Equal(t, "email", repo.feedbackSaved[0].channel)
}

func TestEditMessage_ProspectNotFound_StillSavesFeedback(t *testing.T) {
	msgID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		messages: []*domain.OutboundMessage{
			{ID: msgID, ProspectID: pid, Body: "original", Channel: domain.StepChannelEmail},
		},
	}
	// No prospect in reader
	pr := newMockProspectReader()

	uc := NewUseCase(repo, &mockAI{}, pr, &mockLeadCreator{})
	err := uc.EditMessage(context.Background(), msgID, "edited")
	require.NoError(t, err)

	// Feedback saved with Nil userID since prospect not found
	require.Len(t, repo.feedbackSaved, 1)
	assert.Equal(t, uuid.Nil.String(), repo.feedbackSaved[0].userID)
}

func TestConvertToLead_GetProspectError(t *testing.T) {
	pr := &mockProspectReaderWithErr{getErr: errors.New("db error")}
	uc := NewUseCase(&mockRepo{}, &mockAI{}, pr, &mockLeadCreator{})

	err := uc.ConvertToLead(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get prospect")
}

// mockProspectReaderWithErr returns errors from GetProspect.
type mockProspectReaderWithErr struct {
	getErr    error
	updateErr error
}

func (m *mockProspectReaderWithErr) GetProspect(_ context.Context, _ uuid.UUID) (*domain.ProspectView, error) {
	return nil, m.getErr
}
func (m *mockProspectReaderWithErr) UpdateStatus(_ context.Context, _ uuid.UUID, _ string) error {
	return m.updateErr
}

// Test Launch with prospect reader returning error.
func TestLaunch_GetProspectError(t *testing.T) {
	seqID := uuid.New()
	pid := uuid.New()

	repo := &mockRepo{
		steps: []domain.SequenceStep{
			{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, DelayDays: 0, Channel: domain.StepChannelEmail},
		},
	}

	pr := &mockProspectReaderWithErr{getErr: errors.New("prospect db error")}
	uc := NewUseCase(repo, &mockAI{coldBody: "hi"}, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), seqID, []uuid.UUID{pid})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get prospect")
}

// Test ListSteps through usecase
func TestListSteps(t *testing.T) {
	steps := []domain.SequenceStep{
		{ID: uuid.New(), StepOrder: 1},
		{ID: uuid.New(), StepOrder: 2},
	}
	repo := &mockRepo{steps: steps}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	result, err := uc.ListSteps(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestNewUseCase(t *testing.T) {
	repo := &mockRepo{}
	ai := &mockAI{}
	pr := newMockProspectReader()
	lc := &mockLeadCreator{}

	uc := NewUseCase(repo, ai, pr, lc)
	assert.NotNil(t, uc)

	// With tx manager option
	txMgr := &mockTxManager{}
	uc2 := NewUseCase(repo, ai, pr, lc, WithTxManager(txMgr))
	assert.NotNil(t, uc2)
}
