package leads

import (
	"context"
	"fmt"
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
	updatedSourceIDs      map[uuid.UUID]*uuid.UUID
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

func (m *mockRepo) ListLeads(_ context.Context, _ uuid.UUID) ([]domain.LeadWithSource, error) {
	var result []domain.LeadWithSource
	for _, l := range m.leads {
		result = append(result, domain.LeadWithSource{Lead: *l})
	}
	return result, nil
}

func (m *mockRepo) GetLeadForUser(_ context.Context, userID, leadID uuid.UUID) (*domain.Lead, error) {
	l, ok := m.leads[leadID]
	if !ok || l.UserID != userID {
		return nil, nil
	}
	return l, nil
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

func (m *mockRepo) UpdateSourceID(_ context.Context, id uuid.UUID, sourceID *uuid.UUID) error {
	if m.updatedSourceIDs == nil {
		m.updatedSourceIDs = make(map[uuid.UUID]*uuid.UUID)
	}
	m.updatedSourceIDs[id] = sourceID
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

func TestExportCSV(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	email := "test@example.com"
	repo.leads[leadID] = &domain.Lead{
		ID:           leadID,
		UserID:       userID,
		Channel:      domain.ChannelEmail,
		ContactName:  "Alice",
		Company:      "Acme",
		FirstMessage: "Hello",
		Status:       domain.StatusNew,
		EmailAddress: &email,
	}

	uc := NewUseCase(repo, &mockAI{}, nil)
	data, err := uc.ExportCSV(context.Background(), userID)
	require.NoError(t, err)

	csv := string(data)
	assert.Contains(t, csv, "contact_name,company,channel,email_address")
	assert.Contains(t, csv, "Alice,Acme,email,test@example.com")
}

func TestImportCSV_HappyPath(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, &mockAI{}, nil)
	userID := uuid.New()

	csv := []byte("contact_name,channel,company,email_address\nAlice,email,Acme,alice@example.com\nBob,telegram,,\n")

	count, err := uc.ImportCSV(context.Background(), userID, csv)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Len(t, repo.leads, 2)
}

func TestImportCSV_MissingRequiredColumn(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, &mockAI{}, nil)

	csv := []byte("name,email\nAlice,alice@example.com\n")

	count, err := uc.ImportCSV(context.Background(), uuid.New(), csv)
	assert.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "contact_name")
}

func TestImportCSV_SkipsEmptyName(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, &mockAI{}, nil)

	csv := []byte("contact_name,channel\n,email\nBob,telegram\n")

	count, err := uc.ImportCSV(context.Background(), uuid.New(), csv)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestQualifyLead_HappyPath(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:           leadID,
		ContactName:  "Ivan",
		Channel:      domain.ChannelTelegram,
		FirstMessage: "Need a CRM system",
		Status:       domain.StatusNew,
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

func TestRegenerateDraft_NotFound(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, &mockAI{}, nil)

	d, err := uc.RegenerateDraft(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Nil(t, d)
	assert.Contains(t, err.Error(), "lead not found")
}

func TestRegenerateDraft_WithQualification(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:           leadID,
		ContactName:  "Anna",
		FirstMessage: "Looking for a partner",
	}
	repo.qualifications[leadID] = &domain.Qualification{
		ID:             uuid.New(),
		LeadID:         leadID,
		IdentifiedNeed: "CRM",
		Score:          90,
	}

	aiSvc := &mockAI{draftBody: "Draft with context"}
	uc := NewUseCase(repo, aiSvc, nil)

	d, err := uc.RegenerateDraft(context.Background(), leadID)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, "Draft with context", d.Body)
}

func TestRegenerateDraft_AIError(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:           leadID,
		ContactName:  "Anna",
		FirstMessage: "Looking for a partner",
	}

	aiSvc := &mockAI{draftErr: fmt.Errorf("ai unavailable")}
	uc := NewUseCase(repo, aiSvc, nil)

	d, err := uc.RegenerateDraft(context.Background(), leadID)
	assert.Error(t, err)
	assert.Nil(t, d)
	assert.Contains(t, err.Error(), "ai unavailable")
}

func TestGetMessages(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	repo.messages[leadID] = []domain.Message{
		{ID: uuid.New(), LeadID: leadID, Direction: domain.DirectionInbound, Body: "hi"},
		{ID: uuid.New(), LeadID: leadID, Direction: domain.DirectionOutbound, Body: "hello"},
	}
	uc := NewUseCase(repo, &mockAI{}, nil)

	msgs, err := uc.GetMessages(context.Background(), leadID)
	require.NoError(t, err)
	assert.Len(t, msgs, 2)
}

func TestGetMessages_Empty(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, &mockAI{}, nil)

	msgs, err := uc.GetMessages(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestGetQualification_Found(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	repo.qualifications[leadID] = &domain.Qualification{
		ID:             uuid.New(),
		LeadID:         leadID,
		IdentifiedNeed: "CRM",
		Score:          80,
	}
	uc := NewUseCase(repo, &mockAI{}, nil)

	q, err := uc.GetQualification(context.Background(), leadID)
	require.NoError(t, err)
	require.NotNil(t, q)
	assert.Equal(t, "CRM", q.IdentifiedNeed)
}

func TestGetQualification_NotFound(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, &mockAI{}, nil)

	q, err := uc.GetQualification(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, q)
}

func TestGetDraft_Found(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	repo.drafts[leadID] = &domain.Draft{
		ID:     uuid.New(),
		LeadID: leadID,
		Body:   "draft body",
	}
	uc := NewUseCase(repo, &mockAI{}, nil)

	d, err := uc.GetDraft(context.Background(), leadID)
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, "draft body", d.Body)
}

func TestGetDraft_NotFound(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, &mockAI{}, nil)

	d, err := uc.GetDraft(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, d)
}

func TestSendMessage_AutoTransition(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:      leadID,
		Channel: domain.ChannelEmail,
		Status:  domain.StatusQualified,
	}

	uc := NewUseCase(repo, &mockAI{}, nil)

	msg, err := uc.SendMessage(context.Background(), leadID, "Let's chat")
	require.NoError(t, err)
	require.NotNil(t, msg)

	assert.Equal(t, domain.StatusInConversation, repo.updatedStatuses[leadID])
}

func TestSendMessage_NoAutoTransitionForNonQualified(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:      leadID,
		Channel: domain.ChannelEmail,
		Status:  domain.StatusInConversation,
	}

	uc := NewUseCase(repo, &mockAI{}, nil)

	msg, err := uc.SendMessage(context.Background(), leadID, "Follow up")
	require.NoError(t, err)
	require.NotNil(t, msg)

	_, transitioned := repo.updatedStatuses[leadID]
	assert.False(t, transitioned)
}

func TestSendMessage_WithSender(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	chatID := int64(999)
	repo.leads[leadID] = &domain.Lead{
		ID:             leadID,
		Channel:        domain.ChannelTelegram,
		TelegramChatID: &chatID,
		Status:         domain.StatusInConversation,
	}

	sender := &mockSender{}
	uc := NewUseCase(repo, &mockAI{}, sender)

	msg, err := uc.SendMessage(context.Background(), leadID, "Sent via TG")
	require.NoError(t, err)
	require.NotNil(t, msg)
	require.Len(t, sender.sentMessages, 1)
	assert.Equal(t, "Sent via TG", sender.sentMessages[0].Body)
}

func TestSendMessage_SenderError(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	chatID := int64(999)
	repo.leads[leadID] = &domain.Lead{
		ID:             leadID,
		Channel:        domain.ChannelTelegram,
		TelegramChatID: &chatID,
		Status:         domain.StatusInConversation,
	}

	sender := &mockSenderWithErr{err: fmt.Errorf("telegram down")}
	uc := NewUseCase(repo, &mockAI{}, sender)

	msg, err := uc.SendMessage(context.Background(), leadID, "fail")
	assert.Error(t, err)
	assert.Nil(t, msg)
	assert.Contains(t, err.Error(), "send message")
}

func TestExportCSV_EmptyLeads(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, &mockAI{}, nil)

	data, err := uc.ExportCSV(context.Background(), uuid.New())
	require.NoError(t, err)
	csv := string(data)
	assert.Contains(t, csv, "contact_name,company,channel,email_address")
	// Only header, no data rows besides the header
	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	assert.Equal(t, 1, lines) // just the header line
}

func TestImportCSV_DedupByEmail(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	// Pre-fill an existing lead with the same email
	existingID := uuid.New()
	existingEmail := "alice@example.com"
	repo.leads[existingID] = &domain.Lead{
		ID:           existingID,
		UserID:       userID,
		EmailAddress: &existingEmail,
	}

	// Override GetLeadByEmailAddress to actually find the existing lead
	repoDedup := &mockRepoDedup{mockRepo: newMockRepo(), existingEmails: map[string]*domain.Lead{
		"alice@example.com": repo.leads[existingID],
	}}

	uc := NewUseCase(repoDedup, &mockAI{}, nil)

	csvData := []byte("contact_name,channel,email_address\nAlice,email,alice@example.com\nBob,email,bob@example.com\n")
	count, err := uc.ImportCSV(context.Background(), userID, csvData)
	require.NoError(t, err)
	assert.Equal(t, 1, count) // Alice skipped (dedup), Bob imported
}

func TestImportCSV_BadCSV(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, &mockAI{}, nil)

	// Completely empty data
	count, err := uc.ImportCSV(context.Background(), uuid.New(), []byte(""))
	assert.Error(t, err)
	assert.Equal(t, 0, count)
}

func TestUpdateStatus_InvalidStatus(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, &mockAI{}, nil)

	err := uc.UpdateStatus(context.Background(), uuid.New(), "bogus")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func TestUpdateStatus_LeadNotFound(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, &mockAI{}, nil)

	err := uc.UpdateStatus(context.Background(), uuid.New(), "qualified")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lead not found")
}

func TestUpdateStatus_InvalidTransition(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:     leadID,
		Status: domain.StatusNew,
	}
	uc := NewUseCase(repo, &mockAI{}, nil)

	err := uc.UpdateStatus(context.Background(), leadID, "won")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot transition")
}

func TestUpdateStatus_HappyPath(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:     leadID,
		Status: domain.StatusNew,
	}
	uc := NewUseCase(repo, &mockAI{}, nil)

	err := uc.UpdateStatus(context.Background(), leadID, "qualified")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusQualified, repo.updatedStatuses[leadID])
}

func TestListLeads(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	repo.leads[uuid.New()] = &domain.Lead{ID: uuid.New(), UserID: userID, ContactName: "A"}
	repo.leads[uuid.New()] = &domain.Lead{ID: uuid.New(), UserID: userID, ContactName: "B"}
	uc := NewUseCase(repo, &mockAI{}, nil)

	leads, err := uc.ListLeads(context.Background(), userID)
	require.NoError(t, err)
	assert.Len(t, leads, 2)
}

func TestGetLead_Found(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, ContactName: "Alice"}
	uc := NewUseCase(repo, &mockAI{}, nil)

	lead, err := uc.GetLead(context.Background(), leadID)
	require.NoError(t, err)
	require.NotNil(t, lead)
	assert.Equal(t, "Alice", lead.ContactName)
}

func TestGetLead_NotFound(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, &mockAI{}, nil)

	lead, err := uc.GetLead(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, lead)
}

func TestSetSender(t *testing.T) {
	uc := NewUseCase(newMockRepo(), &mockAI{}, nil)
	assert.Nil(t, uc.sender)

	sender := &mockSender{}
	uc.SetSender(sender)
	assert.NotNil(t, uc.sender)
}

func TestQualifyLead_AIError(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:           leadID,
		ContactName:  "Ivan",
		Channel:      domain.ChannelTelegram,
		FirstMessage: "Need CRM",
		Status:       domain.StatusNew,
	}

	aiSvc := &mockAI{qualifyErr: fmt.Errorf("ai broken")}
	uc := NewUseCase(repo, aiSvc, nil)

	q, err := uc.QualifyLead(context.Background(), leadID)
	assert.Error(t, err)
	assert.Nil(t, q)
}

func TestSendMessage_GetLeadError(t *testing.T) {
	repo := &mockRepoWithGetLeadErr{mockRepo: newMockRepo(), err: fmt.Errorf("db fail")}
	uc := NewUseCase(repo, &mockAI{}, nil)

	msg, err := uc.SendMessage(context.Background(), uuid.New(), "hi")
	assert.Error(t, err)
	assert.Nil(t, msg)
	assert.Contains(t, err.Error(), "get lead")
}

func TestSendMessage_CreateMessageError(t *testing.T) {
	repo := &mockRepoWithCreateMsgErr{mockRepo: newMockRepo(), err: fmt.Errorf("insert fail")}
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:      leadID,
		Channel: domain.ChannelEmail,
		Status:  domain.StatusInConversation,
	}
	uc := NewUseCase(repo, &mockAI{}, nil)

	msg, err := uc.SendMessage(context.Background(), leadID, "hi")
	assert.Error(t, err)
	assert.Nil(t, msg)
}

func TestUpdateStatus_GetLeadError(t *testing.T) {
	repo := &mockRepoWithGetLeadErr{mockRepo: newMockRepo(), err: fmt.Errorf("db fail")}
	uc := NewUseCase(repo, &mockAI{}, nil)

	err := uc.UpdateStatus(context.Background(), uuid.New(), "qualified")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get lead")
}

func TestRegenerateDraft_CreateDraftError(t *testing.T) {
	repo := &mockRepoWithCreateDraftErr{mockRepo: newMockRepo()}
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:           leadID,
		ContactName:  "Anna",
		FirstMessage: "Looking for help",
	}
	uc := NewUseCase(repo, &mockAI{draftBody: "draft"}, nil)

	d, err := uc.RegenerateDraft(context.Background(), leadID)
	assert.Error(t, err)
	assert.Nil(t, d)
}

func TestQualifyLead_UpsertQualificationError(t *testing.T) {
	repo := &mockRepoWithUpsertQualErr{mockRepo: newMockRepo()}
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:           leadID,
		ContactName:  "Ivan",
		Channel:      domain.ChannelTelegram,
		FirstMessage: "Need CRM",
		Status:       domain.StatusNew,
	}
	aiSvc := &mockAI{
		qualifyResult: &domain.Qualification{
			IdentifiedNeed: "CRM",
			Score:          85,
		},
	}
	uc := NewUseCase(repo, aiSvc, nil)

	q, err := uc.QualifyLead(context.Background(), leadID)
	assert.Error(t, err)
	assert.Nil(t, q)
}

func TestSendMessage_AutoTransitionError(t *testing.T) {
	repo := &mockRepoWithUpdateStatusErr{mockRepo: newMockRepo(), err: fmt.Errorf("update fail")}
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:      leadID,
		Channel: domain.ChannelEmail,
		Status:  domain.StatusQualified,
	}
	uc := NewUseCase(repo, &mockAI{}, nil)

	msg, err := uc.SendMessage(context.Background(), leadID, "Hello")
	assert.Error(t, err)
	assert.Nil(t, msg)
	assert.Contains(t, err.Error(), "auto-transition")
}

func TestExportCSV_WithNilEmail(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:           leadID,
		UserID:       userID,
		Channel:      domain.ChannelTelegram,
		ContactName:  "Bob",
		Company:      "Inc",
		FirstMessage: "Hey",
		Status:       domain.StatusNew,
		EmailAddress: nil,
	}

	uc := NewUseCase(repo, &mockAI{}, nil)
	data, err := uc.ExportCSV(context.Background(), userID)
	require.NoError(t, err)

	csv := string(data)
	assert.Contains(t, csv, "Bob,Inc,telegram,,new")
}

func TestQualifyLead_TransitionError(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	// Already qualified => transition new->qualified works, but if already in wrong state
	repo.leads[leadID] = &domain.Lead{
		ID:           leadID,
		ContactName:  "Ivan",
		Channel:      domain.ChannelTelegram,
		FirstMessage: "Need CRM",
		Status:       domain.StatusClosed, // cannot transition from closed
	}

	aiSvc := &mockAI{
		qualifyResult: &domain.Qualification{
			IdentifiedNeed: "CRM",
			Score:          85,
		},
	}
	uc := NewUseCase(repo, aiSvc, nil)

	q, err := uc.QualifyLead(context.Background(), leadID)
	assert.Error(t, err)
	assert.Nil(t, q)
	assert.Contains(t, err.Error(), "cannot transition")
}

func TestImportCSV_MissingChannelColumn(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, &mockAI{}, nil)

	csvData := []byte("contact_name,company\nAlice,Acme\n")
	count, err := uc.ImportCSV(context.Background(), uuid.New(), csvData)
	assert.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "channel")
}

func TestImportCSV_MalformedRow(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, &mockAI{}, nil)

	// Row has wrong number of fields (3 columns header, but row has unquoted comma in field)
	csvData := []byte("contact_name,channel\n\"unclosed quote\n")
	count, err := uc.ImportCSV(context.Background(), uuid.New(), csvData)
	assert.Error(t, err)
	assert.Equal(t, 0, count)
}

func TestQualifyLead_GetLeadError(t *testing.T) {
	repo := &mockRepoWithGetLeadErr{mockRepo: newMockRepo(), err: fmt.Errorf("db fail")}
	uc := NewUseCase(repo, &mockAI{}, nil)

	q, err := uc.QualifyLead(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Nil(t, q)
}

func TestRegenerateDraft_GetLeadError(t *testing.T) {
	repo := &mockRepoWithGetLeadErr{mockRepo: newMockRepo(), err: fmt.Errorf("db fail")}
	uc := NewUseCase(repo, &mockAI{}, nil)

	d, err := uc.RegenerateDraft(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Nil(t, d)
}

func TestImportCSV_CreateLeadError(t *testing.T) {
	repo := &mockRepoWithCreateLeadErr{mockRepo: newMockRepo()}
	uc := NewUseCase(repo, &mockAI{}, nil)

	csvData := []byte("contact_name,channel\nAlice,email\n")
	count, err := uc.ImportCSV(context.Background(), uuid.New(), csvData)
	assert.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "create lead")
}

func TestImportCSV_DedupCheckError(t *testing.T) {
	repo := &mockRepoWithEmailCheckErr{mockRepo: newMockRepo()}
	uc := NewUseCase(repo, &mockAI{}, nil)

	csvData := []byte("contact_name,channel,email_address\nAlice,email,alice@example.com\n")
	count, err := uc.ImportCSV(context.Background(), uuid.New(), csvData)
	assert.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "dedup lead check")
}

func TestExportCSV_ListLeadsError(t *testing.T) {
	repo := &mockRepoWithListErr{mockRepo: newMockRepo(), err: fmt.Errorf("db fail")}
	uc := NewUseCase(repo, &mockAI{}, nil)

	data, err := uc.ExportCSV(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Nil(t, data)
}

func TestQualifyLead_UpdateStatusError(t *testing.T) {
	repo := &mockRepoWithUpdateStatusErr{mockRepo: newMockRepo(), err: fmt.Errorf("update fail")}
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:           leadID,
		ContactName:  "Ivan",
		Channel:      domain.ChannelTelegram,
		FirstMessage: "Need CRM",
		Status:       domain.StatusNew,
	}

	aiSvc := &mockAI{
		qualifyResult: &domain.Qualification{
			IdentifiedNeed: "CRM",
			Score:          85,
		},
	}
	uc := NewUseCase(repo, aiSvc, nil)

	q, err := uc.QualifyLead(context.Background(), leadID)
	assert.Error(t, err)
	assert.Nil(t, q)
}

func TestExportCSV_MultipleLeads(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	email := "a@b.com"
	repo.leads[uuid.New()] = &domain.Lead{
		ID: uuid.New(), UserID: userID, Channel: domain.ChannelEmail,
		ContactName: "Alice", Company: "A", Status: domain.StatusNew, EmailAddress: &email,
	}
	repo.leads[uuid.New()] = &domain.Lead{
		ID: uuid.New(), UserID: userID, Channel: domain.ChannelTelegram,
		ContactName: "Bob", Company: "B", Status: domain.StatusQualified,
	}

	uc := NewUseCase(repo, &mockAI{}, nil)
	data, err := uc.ExportCSV(context.Background(), userID)
	require.NoError(t, err)
	csv := string(data)
	assert.Contains(t, csv, "Alice")
	assert.Contains(t, csv, "Bob")
}

func TestUpdateStatus_UpdateRepoError(t *testing.T) {
	repo := &mockRepoWithUpdateStatusErr{mockRepo: newMockRepo(), err: fmt.Errorf("update fail")}
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, Status: domain.StatusNew}
	uc := NewUseCase(repo, &mockAI{}, nil)

	err := uc.UpdateStatus(context.Background(), leadID, "qualified")
	assert.Error(t, err)
}

func TestRegenerateDraft_GetQualError(t *testing.T) {
	repo := &mockRepoWithGetQualErr{mockRepo: newMockRepo(), err: fmt.Errorf("qual fail")}
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{
		ID:           leadID,
		ContactName:  "Anna",
		FirstMessage: "Looking for help",
	}
	uc := NewUseCase(repo, &mockAI{draftBody: "draft"}, nil)

	d, err := uc.RegenerateDraft(context.Background(), leadID)
	assert.Error(t, err)
	assert.Nil(t, d)
}

// --- Additional mock types ---

type mockSenderWithErr struct {
	err error
}

func (m *mockSenderWithErr) SendMessage(_ context.Context, _ *domain.Lead, _ string) error {
	return m.err
}

type mockRepoDedup struct {
	*mockRepo
	existingEmails map[string]*domain.Lead
}

func (m *mockRepoDedup) GetLeadByEmailAddress(_ context.Context, _ uuid.UUID, email string) (*domain.Lead, error) {
	if l, ok := m.existingEmails[email]; ok {
		return l, nil
	}
	return nil, nil
}

type mockRepoWithGetLeadErr struct {
	*mockRepo
	err error
}

func (m *mockRepoWithGetLeadErr) GetLead(_ context.Context, _ uuid.UUID) (*domain.Lead, error) {
	return nil, m.err
}

type mockRepoWithCreateMsgErr struct {
	*mockRepo
	err error
}

func (m *mockRepoWithCreateMsgErr) CreateMessage(_ context.Context, _ *domain.Message) error {
	return m.err
}

type mockRepoWithListErr struct {
	*mockRepo
	err error
}

func (m *mockRepoWithListErr) ListLeads(_ context.Context, _ uuid.UUID) ([]domain.LeadWithSource, error) {
	return nil, m.err
}

type mockRepoWithUpdateStatusErr struct {
	*mockRepo
	err error
}

func (m *mockRepoWithUpdateStatusErr) UpdateLeadStatus(_ context.Context, _ uuid.UUID, _ domain.LeadStatus) error {
	return m.err
}

type mockRepoWithGetQualErr struct {
	*mockRepo
	err error
}

func (m *mockRepoWithGetQualErr) GetQualification(_ context.Context, _ uuid.UUID) (*domain.Qualification, error) {
	return nil, m.err
}

type mockRepoWithCreateDraftErr struct {
	*mockRepo
}

func (m *mockRepoWithCreateDraftErr) CreateDraft(_ context.Context, _ *domain.Draft) error {
	return fmt.Errorf("create draft fail")
}

type mockRepoWithUpsertQualErr struct {
	*mockRepo
}

func (m *mockRepoWithUpsertQualErr) UpsertQualification(_ context.Context, _ *domain.Qualification) error {
	return fmt.Errorf("upsert fail")
}

type mockRepoWithCreateLeadErr struct {
	*mockRepo
}

func (m *mockRepoWithCreateLeadErr) CreateLead(_ context.Context, _ *domain.Lead) error {
	return fmt.Errorf("create lead fail")
}

type mockRepoWithEmailCheckErr struct {
	*mockRepo
}

func (m *mockRepoWithEmailCheckErr) GetLeadByEmailAddress(_ context.Context, _ uuid.UUID, _ string) (*domain.Lead, error) {
	return nil, fmt.Errorf("email check fail")
}
