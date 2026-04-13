package inbox

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/leads/domain"
)

// fakeHTTPClient returns a canned Telegram API response so BotAPI.Send doesn't panic.
type fakeHTTPClient struct{}

func (f *fakeHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	body := `{"ok":true,"result":{"message_id":1,"chat":{"id":1}}}`
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}, nil
}

// --- Mock LeadRepository ---

type mockLeadRepo struct {
	mu                    sync.Mutex
	leads                 []*domain.Lead
	messages              []*domain.Message
	qualifications        []*domain.Qualification
	updatedStatuses       map[uuid.UUID]domain.LeadStatus
	updatedFirstMessages  map[uuid.UUID]string
	existingLeadByChatID  map[int64]*domain.Lead // preset for GetLeadByTelegramChatID
	qualifyDone           chan struct{}
}

func newMockLeadRepo() *mockLeadRepo {
	return &mockLeadRepo{
		updatedStatuses:      make(map[uuid.UUID]domain.LeadStatus),
		updatedFirstMessages: make(map[uuid.UUID]string),
		existingLeadByChatID: make(map[int64]*domain.Lead),
		qualifyDone:          make(chan struct{}, 1),
	}
}

func (m *mockLeadRepo) GetLeadByTelegramChatID(_ context.Context, _ uuid.UUID, chatID int64) (*domain.Lead, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if lead, ok := m.existingLeadByChatID[chatID]; ok {
		return lead, nil
	}
	return nil, nil
}

func (m *mockLeadRepo) GetLeadByEmailAddress(_ context.Context, _ uuid.UUID, _ string) (*domain.Lead, error) {
	return nil, nil
}

func (m *mockLeadRepo) CreateLead(_ context.Context, lead *domain.Lead) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.leads = append(m.leads, lead)
	return nil
}

func (m *mockLeadRepo) UpdateFirstMessage(_ context.Context, id uuid.UUID, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updatedFirstMessages[id] = message
	return nil
}

func (m *mockLeadRepo) CreateMessage(_ context.Context, msg *domain.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockLeadRepo) UpsertQualification(_ context.Context, q *domain.Qualification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.qualifications = append(m.qualifications, q)
	return nil
}

func (m *mockLeadRepo) UpdateLeadStatus(_ context.Context, id uuid.UUID, status domain.LeadStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updatedStatuses[id] = status
	select {
	case m.qualifyDone <- struct{}{}:
	default:
	}
	return nil
}

// --- Mock AIQualifier ---

type mockAIQualifier struct {
	result *ai.QualificationResult
}

func (m *mockAIQualifier) Qualify(_ context.Context, _, _, _ string) (*ai.QualificationResult, error) {
	return m.result, nil
}

func (m *mockAIQualifier) ProviderName() string {
	return "mock"
}

// --- Helpers ---

func newTestBot(repo LeadRepository, aiClient AIQualifier, ownerID uuid.UUID, bookingLink string) *TelegramBot {
	return newTestBotWithProspects(repo, nil, aiClient, ownerID, bookingLink)
}

func newTestBotWithProspects(repo LeadRepository, prospectRepo ProspectRepository, aiClient AIQualifier, ownerID uuid.UUID, bookingLink string) *TelegramBot {
	fakeBotAPI := &tgbotapi.BotAPI{
		Token:  "test-token",
		Client: &fakeHTTPClient{},
	}
	fakeBotAPI.SetAPIEndpoint("http://localhost/bot%s/%s")
	return &TelegramBot{
		bot:          fakeBotAPI,
		repo:         repo,
		prospectRepo: prospectRepo,
		aiClient:     aiClient,
		ownerID:      ownerID,
		bookingLink:  bookingLink,
	}
}

// --- Mock ProspectRepository ---

type mockProspectRepo struct {
	mu        sync.Mutex
	byTgUser  map[string]*ProspectMatch
	converted []uuid.UUID
}

func newMockProspectRepo() *mockProspectRepo {
	return &mockProspectRepo{
		byTgUser: make(map[string]*ProspectMatch),
	}
}

func (m *mockProspectRepo) FindByEmail(_ context.Context, _ uuid.UUID, _ string) (*ProspectMatch, error) {
	return nil, nil
}

func (m *mockProspectRepo) FindByTelegramUsername(_ context.Context, _ uuid.UUID, username string) (*ProspectMatch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.byTgUser[username], nil
}

func (m *mockProspectRepo) ConvertToLead(_ context.Context, prospectID, _ uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.converted = append(m.converted, prospectID)
	return nil
}

func makeTgMessage(chatID int64, firstName, lastName, text string) *tgbotapi.Message {
	return &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: chatID},
		From: &tgbotapi.User{FirstName: firstName, LastName: lastName},
		Text: text,
	}
}

func makeTgMessageWithUsername(chatID int64, firstName, lastName, username, text string) *tgbotapi.Message {
	return &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: chatID},
		From: &tgbotapi.User{FirstName: firstName, LastName: lastName, UserName: username},
		Text: text,
	}
}

// waitQualifyDone waits for the async qualification goroutine to finish,
// or fails the test after a timeout.
func waitQualifyDone(t *testing.T, repo *mockLeadRepo) {
	t.Helper()
	select {
	case <-repo.qualifyDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for async qualification to complete")
	}
}

// --- Tests ---

func TestHandleMessage_NewLead(t *testing.T) {
	repo := newMockLeadRepo()
	aiClient := &mockAIQualifier{
		result: &ai.QualificationResult{
			IdentifiedNeed: "CRM system",
			Score:          7,
		},
	}
	ownerID := uuid.New()
	bot := newTestBot(repo, aiClient, ownerID, "https://cal.com/test")

	msg := makeTgMessage(12345, "Ivan", "Petrov", "Hello, I need a CRM")
	bot.handleMessage(context.Background(), msg)

	// Wait for the async qualification goroutine to complete.
	waitQualifyDone(t, repo)

	repo.mu.Lock()
	defer repo.mu.Unlock()

	// A new lead should have been created.
	require.Len(t, repo.leads, 1)
	lead := repo.leads[0]
	assert.Equal(t, ownerID, lead.UserID)
	assert.Equal(t, domain.ChannelTelegram, lead.Channel)
	assert.Equal(t, "Ivan Petrov", lead.ContactName)
	assert.Equal(t, "Hello, I need a CRM", lead.FirstMessage)
	assert.Equal(t, domain.StatusNew, lead.Status)
	require.NotNil(t, lead.TelegramChatID)
	assert.Equal(t, int64(12345), *lead.TelegramChatID)

	// An inbound message should have been created.
	require.Len(t, repo.messages, 1)
	assert.Equal(t, lead.ID, repo.messages[0].LeadID)
	assert.Equal(t, domain.DirectionInbound, repo.messages[0].Direction)
	assert.Equal(t, "Hello, I need a CRM", repo.messages[0].Body)

	// Qualification should have run asynchronously.
	require.Len(t, repo.qualifications, 1)
	assert.Equal(t, lead.ID, repo.qualifications[0].LeadID)
	assert.Equal(t, "CRM system", repo.qualifications[0].IdentifiedNeed)
	assert.Equal(t, 7, repo.qualifications[0].Score)
	assert.Equal(t, "mock", repo.qualifications[0].ProviderUsed)

	// Lead status should have been updated to qualified.
	assert.Equal(t, domain.StatusQualified, repo.updatedStatuses[lead.ID])
}

func TestHandleMessage_ExistingLead(t *testing.T) {
	repo := newMockLeadRepo()
	aiClient := &mockAIQualifier{
		result: &ai.QualificationResult{Score: 5},
	}
	ownerID := uuid.New()
	existingLead := &domain.Lead{
		ID:             uuid.New(),
		UserID:         ownerID,
		Channel:        domain.ChannelTelegram,
		ContactName:    "Ivan Petrov",
		FirstMessage:   "hi",
		Status:         domain.StatusNew,
		TelegramChatID: ptrInt64(99999),
	}
	repo.existingLeadByChatID[99999] = existingLead

	bot := newTestBot(repo, aiClient, ownerID, "https://cal.com/test")

	msg := makeTgMessage(99999, "Ivan", "Petrov", "Actually, I need a full ERP system for my company")
	bot.handleMessage(context.Background(), msg)

	// Wait for the async qualification goroutine to complete.
	waitQualifyDone(t, repo)

	repo.mu.Lock()
	defer repo.mu.Unlock()

	// No new lead should have been created.
	assert.Empty(t, repo.leads)

	// FirstMessage should have been updated (old was short "hi", new is long).
	assert.Equal(t, "Actually, I need a full ERP system for my company", repo.updatedFirstMessages[existingLead.ID])

	// An inbound message should still be created.
	require.Len(t, repo.messages, 1)
	assert.Equal(t, existingLead.ID, repo.messages[0].LeadID)
	assert.Equal(t, domain.DirectionInbound, repo.messages[0].Direction)
}

func TestHandleMessage_CallAgreement(t *testing.T) {
	repo := newMockLeadRepo()
	aiClient := &mockAIQualifier{
		result: &ai.QualificationResult{Score: 9},
	}
	ownerID := uuid.New()
	bookingLink := "https://cal.com/booking"
	bot := newTestBot(repo, aiClient, ownerID, bookingLink)

	// "давайте созвон" triggers call agreement detection.
	msg := makeTgMessage(77777, "Anna", "", "Звучит интересно, давайте созвон проведём!")
	bot.handleMessage(context.Background(), msg)

	// Wait for the async qualification goroutine to complete.
	waitQualifyDone(t, repo)

	repo.mu.Lock()
	defer repo.mu.Unlock()

	// Lead should be created (new chat).
	require.Len(t, repo.leads, 1)
	lead := repo.leads[0]
	assert.Equal(t, "Anna", lead.ContactName)

	// We expect the inbound message + an outbound booking link message.
	var inbound, outbound []*domain.Message
	for _, m := range repo.messages {
		switch m.Direction {
		case domain.DirectionInbound:
			inbound = append(inbound, m)
		case domain.DirectionOutbound:
			outbound = append(outbound, m)
		}
	}
	require.Len(t, inbound, 1)
	assert.Equal(t, lead.ID, inbound[0].LeadID)

	require.Len(t, outbound, 1)
	assert.Equal(t, lead.ID, outbound[0].LeadID)
	assert.Contains(t, outbound[0].Body, bookingLink)
}

func TestHandleMessage_EmptyText(t *testing.T) {
	repo := newMockLeadRepo()
	aiClient := &mockAIQualifier{result: &ai.QualificationResult{}}
	bot := newTestBot(repo, aiClient, uuid.New(), "")

	msg := makeTgMessage(11111, "Test", "", "")
	bot.handleMessage(context.Background(), msg)

	repo.mu.Lock()
	defer repo.mu.Unlock()

	// Nothing should happen for an empty message.
	assert.Empty(t, repo.leads)
	assert.Empty(t, repo.messages)
}

func TestHandleMessage_ProspectAutoConversion(t *testing.T) {
	repo := newMockLeadRepo()
	prospectRepo := newMockProspectRepo()
	ownerID := uuid.New()

	prospectID := uuid.New()
	srcID := uuid.New()
	prospectRepo.byTgUser["testuser"] = &ProspectMatch{
		ID:       prospectID,
		Name:     "Тест Проспект",
		Company:  "TestCo",
		SourceID: &srcID,
		Status:   "new",
	}

	aiClient := &mockAIQualifier{result: &ai.QualificationResult{Score: 5}}
	bot := newTestBotWithProspects(repo, prospectRepo, aiClient, ownerID, "")

	msg := makeTgMessageWithUsername(99999, "Test", "User", "testuser", "Привет, хочу узнать о продукте")
	bot.handleMessage(context.Background(), msg)

	waitQualifyDone(t, repo)

	repo.mu.Lock()
	defer repo.mu.Unlock()

	require.Len(t, repo.leads, 1)
	lead := repo.leads[0]
	assert.Equal(t, "Тест Проспект", lead.ContactName)
	assert.Equal(t, "TestCo", lead.Company)
	assert.Equal(t, &srcID, lead.SourceID)

	prospectRepo.mu.Lock()
	defer prospectRepo.mu.Unlock()
	require.Len(t, prospectRepo.converted, 1)
	assert.Equal(t, prospectID, prospectRepo.converted[0])
}

func TestHandleMessage_ProspectAlreadyConverted(t *testing.T) {
	repo := newMockLeadRepo()
	prospectRepo := newMockProspectRepo()
	ownerID := uuid.New()

	prospectRepo.byTgUser["converteduser"] = &ProspectMatch{
		ID:     uuid.New(),
		Name:   "Already Converted",
		Status: "converted",
	}

	aiClient := &mockAIQualifier{result: &ai.QualificationResult{Score: 5}}
	bot := newTestBotWithProspects(repo, prospectRepo, aiClient, ownerID, "")

	msg := makeTgMessageWithUsername(88888, "Conv", "User", "converteduser", "Привет снова")
	bot.handleMessage(context.Background(), msg)

	waitQualifyDone(t, repo)

	repo.mu.Lock()
	defer repo.mu.Unlock()

	require.Len(t, repo.leads, 1)
	assert.Equal(t, "Conv User", repo.leads[0].ContactName)

	prospectRepo.mu.Lock()
	defer prospectRepo.mu.Unlock()
	assert.Empty(t, prospectRepo.converted)
}

func ptrInt64(v int64) *int64 {
	return &v
}
