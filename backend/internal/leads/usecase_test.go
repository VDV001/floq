package leads

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock Repository ---

type mockRepo struct {
	leads          map[uuid.UUID]*domain.Lead
	messages       map[uuid.UUID][]domain.Message
	qualifications map[uuid.UUID]*domain.Qualification
	drafts         map[uuid.UUID]*domain.Draft

	createdMessages       []*domain.Message
	upsertedQualification *domain.Qualification
	updatedStatuses       map[uuid.UUID]domain.LeadStatus
	createdDrafts         []*domain.Draft
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		leads:           make(map[uuid.UUID]*domain.Lead),
		messages:        make(map[uuid.UUID][]domain.Message),
		qualifications:  make(map[uuid.UUID]*domain.Qualification),
		drafts:          make(map[uuid.UUID]*domain.Draft),
		updatedStatuses: make(map[uuid.UUID]domain.LeadStatus),
	}
}

func (m *mockRepo) ListLeads(_ context.Context, _ uuid.UUID) ([]domain.Lead, error) {
	var result []domain.Lead
	for _, l := range m.leads {
		result = append(result, *l)
	}
	return result, nil
}

func (m *mockRepo) GetLead(_ context.Context, id uuid.UUID) (*domain.Lead, error) {
	l, ok := m.leads[id]
	if !ok {
		return nil, nil
	}
	return l, nil
}

func (m *mockRepo) CreateLead(_ context.Context, lead *domain.Lead) error {
	m.leads[lead.ID] = lead
	return nil
}

func (m *mockRepo) UpdateFirstMessage(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

func (m *mockRepo) UpdateLeadStatus(_ context.Context, id uuid.UUID, status domain.LeadStatus) error {
	m.updatedStatuses[id] = status
	return nil
}

func (m *mockRepo) GetLeadByTelegramChatID(_ context.Context, _ uuid.UUID, _ int64) (*domain.Lead, error) {
	return nil, nil
}

func (m *mockRepo) GetLeadByEmailAddress(_ context.Context, _ uuid.UUID, _ string) (*domain.Lead, error) {
	return nil, nil
}

func (m *mockRepo) StaleLeadsWithoutReminder(_ context.Context, _ int) ([]domain.Lead, error) {
	return nil, nil
}

func (m *mockRepo) ListMessages(_ context.Context, leadID uuid.UUID) ([]domain.Message, error) {
	return m.messages[leadID], nil
}

func (m *mockRepo) CreateMessage(_ context.Context, msg *domain.Message) error {
	m.createdMessages = append(m.createdMessages, msg)
	return nil
}

func (m *mockRepo) GetQualification(_ context.Context, leadID uuid.UUID) (*domain.Qualification, error) {
	return m.qualifications[leadID], nil
}

func (m *mockRepo) UpsertQualification(_ context.Context, q *domain.Qualification) error {
	m.upsertedQualification = q
	m.qualifications[q.LeadID] = q
	return nil
}

func (m *mockRepo) GetLatestDraft(_ context.Context, leadID uuid.UUID) (*domain.Draft, error) {
	return m.drafts[leadID], nil
}

func (m *mockRepo) CreateDraft(_ context.Context, d *domain.Draft) error {
	m.createdDrafts = append(m.createdDrafts, d)
	m.drafts[d.LeadID] = d
	return nil
}

func (m *mockRepo) CreateReminder(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

func (m *mockRepo) CountMonthLeads(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, nil
}

func (m *mockRepo) CountTotalLeads(_ context.Context, _ uuid.UUID) (int, error) {
	return 0, nil
}

// --- Mock AIService ---

type mockAI struct {
	qualifyResult *domain.Qualification
	qualifyErr    error
	draftBody     string
	draftErr      error
}

func (m *mockAI) Qualify(_ context.Context, _ string, _ domain.Channel, _ string) (*domain.Qualification, error) {
	return m.qualifyResult, m.qualifyErr
}

func (m *mockAI) DraftReply(_ context.Context, _ string, _ string) (string, error) {
	return m.draftBody, m.draftErr
}

func (m *mockAI) GenerateFollowup(_ context.Context, _ string, _ string, _ int) (string, error) {
	return "", nil
}

// --- Mock MessageSender ---

type mockSender struct {
	sentMessages []struct {
		Lead *domain.Lead
		Body string
	}
}

func (m *mockSender) SendMessage(_ context.Context, lead *domain.Lead, body string) error {
	m.sentMessages = append(m.sentMessages, struct {
		Lead *domain.Lead
		Body string
	}{lead, body})
	return nil
}

// --- Tests ---

func TestQualifyLead_HappyPath(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:           leadID,
		ContactName:  "Ivan",
		Channel:      domain.ChannelTelegram,
		FirstMessage: "Need a CRM system",
	}

	aiSvc := &mockAI{
		qualifyResult: &domain.Qualification{
			IdentifiedNeed:    "CRM system",
			EstimatedBudget:   "50000 RUB",
			Deadline:          "2 weeks",
			Score:             85,
			ScoreReason:       "High intent",
			RecommendedAction: "Schedule a demo",
			ProviderUsed:      "test",
		},
	}

	uc := NewUseCase(repo, aiSvc, nil)

	q, err := uc.QualifyLead(context.Background(), leadID)
	require.NoError(t, err)
	require.NotNil(t, q)

	assert.Equal(t, "CRM system", q.IdentifiedNeed)
	assert.Equal(t, 85, q.Score)
	assert.Equal(t, leadID, q.LeadID)
	assert.NotEqual(t, uuid.Nil, q.ID)

	// Check that status was updated to qualified
	assert.Equal(t, domain.StatusQualified, repo.updatedStatuses[leadID])

	// Check that qualification was persisted
	assert.NotNil(t, repo.upsertedQualification)
}

func TestQualifyLead_NotFound(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, &mockAI{}, nil)

	q, err := uc.QualifyLead(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Nil(t, q)
	assert.Contains(t, err.Error(), "lead not found")
}

func TestSendMessage_HappyPath_Telegram(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	chatID := int64(12345)
	repo.leads[leadID] = &domain.Lead{
		ID:             leadID,
		Channel:        domain.ChannelTelegram,
		TelegramChatID: &chatID,
	}

	sender := &mockSender{}
	uc := NewUseCase(repo, &mockAI{}, sender)

	msg, err := uc.SendMessage(context.Background(), leadID, "Hello!")
	require.NoError(t, err)
	require.NotNil(t, msg)

	assert.Equal(t, "Hello!", msg.Body)
	assert.Equal(t, domain.DirectionOutbound, msg.Direction)
	assert.Equal(t, leadID, msg.LeadID)

	// Check that sender was called
	require.Len(t, sender.sentMessages, 1)
	assert.Equal(t, "Hello!", sender.sentMessages[0].Body)

	// Check that message was persisted
	require.Len(t, repo.createdMessages, 1)
}

func TestSendMessage_NilSender(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	chatID := int64(12345)
	repo.leads[leadID] = &domain.Lead{
		ID:             leadID,
		Channel:        domain.ChannelTelegram,
		TelegramChatID: &chatID,
	}

	uc := NewUseCase(repo, &mockAI{}, nil)

	msg, err := uc.SendMessage(context.Background(), leadID, "Hello!")
	require.NoError(t, err)
	require.NotNil(t, msg)

	// Message should still be persisted even without a sender
	require.Len(t, repo.createdMessages, 1)
	assert.Equal(t, "Hello!", repo.createdMessages[0].Body)
}

func TestRegenerateDraft_HappyPath(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:           leadID,
		ContactName:  "Anna",
		FirstMessage: "Looking for a partner",
	}

	aiSvc := &mockAI{draftBody: "Dear Anna, thanks for reaching out!"}
	uc := NewUseCase(repo, aiSvc, nil)

	d, err := uc.RegenerateDraft(context.Background(), leadID)
	require.NoError(t, err)
	require.NotNil(t, d)

	assert.Equal(t, "Dear Anna, thanks for reaching out!", d.Body)
	assert.Equal(t, leadID, d.LeadID)
	require.Len(t, repo.createdDrafts, 1)
}
