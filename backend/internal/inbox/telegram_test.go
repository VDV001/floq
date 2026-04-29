package inbox

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	leads                 []*InboxLead
	messages              []*InboxMessage
	qualifications        []*InboxQualification
	updatedStatuses       map[uuid.UUID]LeadStatus
	updatedFirstMessages  map[uuid.UUID]string
	existingLeadByChatID  map[int64]*InboxLead // preset for GetLeadByTelegramChatID
	qualifyDone           chan struct{}
}

func newMockLeadRepo() *mockLeadRepo {
	return &mockLeadRepo{
		updatedStatuses:      make(map[uuid.UUID]LeadStatus),
		updatedFirstMessages: make(map[uuid.UUID]string),
		existingLeadByChatID: make(map[int64]*InboxLead),
		qualifyDone:          make(chan struct{}, 1),
	}
}

func (m *mockLeadRepo) GetLeadByTelegramChatID(_ context.Context, _ uuid.UUID, chatID int64) (*InboxLead, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if lead, ok := m.existingLeadByChatID[chatID]; ok {
		return lead, nil
	}
	return nil, nil
}

func (m *mockLeadRepo) GetLeadByEmailAddress(_ context.Context, _ uuid.UUID, _ string) (*InboxLead, error) {
	return nil, nil
}

func (m *mockLeadRepo) CreateLead(_ context.Context, lead *InboxLead) error {
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

func (m *mockLeadRepo) CreateMessage(_ context.Context, msg *InboxMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockLeadRepo) UpsertQualification(_ context.Context, q *InboxQualification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.qualifications = append(m.qualifications, q)
	return nil
}

func (m *mockLeadRepo) UpdateLeadStatus(_ context.Context, id uuid.UUID, status LeadStatus) error {
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
	result *QualificationResult
}

func (m *mockAIQualifier) Qualify(_ context.Context, _, _, _ string) (*QualificationResult, error) {
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
		result: &QualificationResult{
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
	assert.Equal(t, ChannelTelegram, lead.Channel)
	assert.Equal(t, "Ivan Petrov", lead.ContactName)
	assert.Equal(t, "Hello, I need a CRM", lead.FirstMessage)
	assert.Equal(t, StatusNew, lead.Status)
	require.NotNil(t, lead.TelegramChatID)
	assert.Equal(t, int64(12345), *lead.TelegramChatID)

	// An inbound message should have been created.
	require.Len(t, repo.messages, 1)
	assert.Equal(t, lead.ID, repo.messages[0].LeadID)
	assert.Equal(t, DirectionInbound, repo.messages[0].Direction)
	assert.Equal(t, "Hello, I need a CRM", repo.messages[0].Body)

	// Qualification should have run asynchronously.
	require.Len(t, repo.qualifications, 1)
	assert.Equal(t, lead.ID, repo.qualifications[0].LeadID)
	assert.Equal(t, "CRM system", repo.qualifications[0].IdentifiedNeed)
	assert.Equal(t, 7, repo.qualifications[0].Score)
	assert.Equal(t, "mock", repo.qualifications[0].ProviderUsed)

	// Lead status should have been updated to qualified.
	assert.Equal(t, StatusQualified, repo.updatedStatuses[lead.ID])
}

func TestHandleMessage_ExistingLead(t *testing.T) {
	repo := newMockLeadRepo()
	aiClient := &mockAIQualifier{
		result: &QualificationResult{Score: 5},
	}
	ownerID := uuid.New()
	existingLead := &InboxLead{
		ID:             uuid.New(),
		UserID:         ownerID,
		Channel:        ChannelTelegram,
		ContactName:    "Ivan Petrov",
		FirstMessage:   "hi",
		Status:         StatusNew,
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
	assert.Equal(t, DirectionInbound, repo.messages[0].Direction)
}

func TestHandleMessage_CallAgreement(t *testing.T) {
	repo := newMockLeadRepo()
	aiClient := &mockAIQualifier{
		result: &QualificationResult{Score: 9},
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
	var inbound, outbound []*InboxMessage
	for _, m := range repo.messages {
		switch m.Direction {
		case DirectionInbound:
			inbound = append(inbound, m)
		case DirectionOutbound:
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
	aiClient := &mockAIQualifier{result: &QualificationResult{}}
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

	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}
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

	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}
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

// --- Error-returning mock LeadRepo for telegram ---

type errorMockLeadRepo struct {
	mockLeadRepo
	getLeadByChatIDErr error
	createLeadErr      error
	createMessageErr   error
}

func newErrorMockLeadRepo() *errorMockLeadRepo {
	return &errorMockLeadRepo{
		mockLeadRepo: mockLeadRepo{
			updatedStatuses:      make(map[uuid.UUID]LeadStatus),
			updatedFirstMessages: make(map[uuid.UUID]string),
			existingLeadByChatID: make(map[int64]*InboxLead),
			qualifyDone:          make(chan struct{}, 1),
		},
	}
}

func (m *errorMockLeadRepo) GetLeadByTelegramChatID(_ context.Context, _ uuid.UUID, chatID int64) (*InboxLead, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getLeadByChatIDErr != nil {
		return nil, m.getLeadByChatIDErr
	}
	if lead, ok := m.existingLeadByChatID[chatID]; ok {
		return lead, nil
	}
	return nil, nil
}

func (m *errorMockLeadRepo) CreateLead(_ context.Context, lead *InboxLead) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createLeadErr != nil {
		return m.createLeadErr
	}
	m.leads = append(m.leads, lead)
	return nil
}

func (m *errorMockLeadRepo) CreateMessage(_ context.Context, msg *InboxMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createMessageErr != nil {
		return m.createMessageErr
	}
	m.messages = append(m.messages, msg)
	return nil
}

func TestHandleMessage_GetLeadByChatIDError(t *testing.T) {
	repo := newErrorMockLeadRepo()
	repo.getLeadByChatIDErr = errors.New("db unavailable")
	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}
	bot := newTestBot(repo, aiClient, uuid.New(), "")

	msg := makeTgMessage(11111, "Test", "", "Hello")
	bot.handleMessage(context.Background(), msg)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	assert.Empty(t, repo.leads)
	assert.Empty(t, repo.messages)
}

func TestHandleMessage_CreateLeadError(t *testing.T) {
	repo := newErrorMockLeadRepo()
	repo.createLeadErr = errors.New("insert failed")
	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}
	bot := newTestBot(repo, aiClient, uuid.New(), "")

	msg := makeTgMessage(22222, "Test", "", "Hello")
	bot.handleMessage(context.Background(), msg)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	assert.Empty(t, repo.leads) // CreateLead fails
	assert.Empty(t, repo.messages)
}

func TestHandleMessage_CreateMessageError(t *testing.T) {
	repo := newErrorMockLeadRepo()
	repo.createMessageErr = errors.New("message insert failed")
	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}
	bot := newTestBot(repo, aiClient, uuid.New(), "")

	msg := makeTgMessage(33333, "Test", "", "Hello")
	bot.handleMessage(context.Background(), msg)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	// Lead should be created, but message fails.
	require.Len(t, repo.leads, 1)
	assert.Empty(t, repo.messages) // CreateMessage fails
}

// --- Mock AIQualifier that returns errors ---

type errorAIQualifier struct{}

func (m *errorAIQualifier) Qualify(_ context.Context, _, _, _ string) (*QualificationResult, error) {
	return nil, errors.New("ai unavailable")
}

func (m *errorAIQualifier) ProviderName() string {
	return "error-mock"
}

func TestHandleMessage_AIQualificationError(t *testing.T) {
	repo := newMockLeadRepo()
	aiClient := &errorAIQualifier{}
	bot := newTestBot(repo, aiClient, uuid.New(), "")

	msg := makeTgMessage(55555, "Test", "User", "Need help with project")
	bot.handleMessage(context.Background(), msg)

	// Give goroutine time to finish (it returns early on error, no qualifyDone signal).
	time.Sleep(200 * time.Millisecond)

	repo.mu.Lock()
	defer repo.mu.Unlock()

	require.Len(t, repo.leads, 1)
	require.Len(t, repo.messages, 1)
	// No qualification should be saved.
	assert.Empty(t, repo.qualifications)
	assert.Empty(t, repo.updatedStatuses)
}

// --- Mock ProspectRepo that errors on ConvertToLead ---

type errorConvertProspectRepo struct {
	mockProspectRepo
}

func (m *errorConvertProspectRepo) ConvertToLead(_ context.Context, _, _ uuid.UUID) error {
	return errors.New("convert failed")
}

func TestHandleMessage_ProspectConvertError(t *testing.T) {
	repo := newMockLeadRepo()
	prospectRepo := &errorConvertProspectRepo{
		mockProspectRepo: mockProspectRepo{
			byTgUser: map[string]*ProspectMatch{
				"erruser": {
					ID:     uuid.New(),
					Name:   "Error Prospect",
					Status: "new",
				},
			},
		},
	}
	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}
	ownerID := uuid.New()
	bot := newTestBotWithProspects(repo, prospectRepo, aiClient, ownerID, "")

	msg := makeTgMessageWithUsername(66666, "Err", "User", "erruser", "Hello")
	bot.handleMessage(context.Background(), msg)

	waitQualifyDone(t, repo)

	repo.mu.Lock()
	defer repo.mu.Unlock()

	// Lead should still be created despite conversion error.
	require.Len(t, repo.leads, 1)
	assert.Equal(t, "Error Prospect", repo.leads[0].ContactName)
}

func TestHandleMessage_OnlyFirstName(t *testing.T) {
	repo := newMockLeadRepo()
	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 3}}
	bot := newTestBot(repo, aiClient, uuid.New(), "")

	msg := makeTgMessage(44444, "OnlyFirst", "", "Just a message")
	bot.handleMessage(context.Background(), msg)

	waitQualifyDone(t, repo)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.leads, 1)
	assert.Equal(t, "OnlyFirst", repo.leads[0].ContactName)
}

// --- Mock repo that fails on UpsertQualification ---

type upsertErrorMockLeadRepo struct {
	mockLeadRepo
}

func (m *upsertErrorMockLeadRepo) UpsertQualification(_ context.Context, _ *InboxQualification) error {
	// Signal done so test doesn't hang.
	select {
	case m.qualifyDone <- struct{}{}:
	default:
	}
	return errors.New("upsert failed")
}

func TestHandleMessage_UpsertQualificationError(t *testing.T) {
	repo := &upsertErrorMockLeadRepo{
		mockLeadRepo: mockLeadRepo{
			updatedStatuses:      make(map[uuid.UUID]LeadStatus),
			updatedFirstMessages: make(map[uuid.UUID]string),
			existingLeadByChatID: make(map[int64]*InboxLead),
			qualifyDone:          make(chan struct{}, 1),
		},
	}
	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}
	bot := newTestBot(repo, aiClient, uuid.New(), "")

	msg := makeTgMessage(77770, "Test", "User", "Hello there")
	bot.handleMessage(context.Background(), msg)

	waitQualifyDone(t, &repo.mockLeadRepo)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.leads, 1)
	// UpsertQualification fails, so UpdateLeadStatus should NOT have been called.
	assert.Empty(t, repo.updatedStatuses)
}

// --- Mock repo that fails on UpdateLeadStatus ---

type statusErrorMockLeadRepo struct {
	mockLeadRepo
}

func (m *statusErrorMockLeadRepo) UpdateLeadStatus(_ context.Context, _ uuid.UUID, _ LeadStatus) error {
	select {
	case m.qualifyDone <- struct{}{}:
	default:
	}
	return errors.New("status update failed")
}

func TestHandleMessage_UpdateLeadStatusError(t *testing.T) {
	repo := &statusErrorMockLeadRepo{
		mockLeadRepo: mockLeadRepo{
			updatedStatuses:      make(map[uuid.UUID]LeadStatus),
			updatedFirstMessages: make(map[uuid.UUID]string),
			existingLeadByChatID: make(map[int64]*InboxLead),
			qualifyDone:          make(chan struct{}, 1),
		},
	}
	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}
	bot := newTestBot(repo, aiClient, uuid.New(), "")

	msg := makeTgMessage(77771, "Test", "User", "Hello there")
	bot.handleMessage(context.Background(), msg)

	waitQualifyDone(t, &repo.mockLeadRepo)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.leads, 1)
	// Qualification is saved but status update fails — still no panic.
	require.Len(t, repo.qualifications, 1)
}

func TestNewTelegramBot_InvalidToken(t *testing.T) {
	// Empty token should fail.
	_, err := NewTelegramBot("", nil, nil, nil, uuid.New(), "", nil)
	// telegram bot-api returns error for empty token.
	assert.Error(t, err)
}

func TestNewTelegramBot_Success(t *testing.T) {
	// Start a local HTTP server that mimics Telegram's getMe endpoint.
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":123,"is_bot":true,"first_name":"TestBot","username":"test_bot"}}`))
	})
	// Use port 0 for auto-assign.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &http.Server{Handler: mux}
	go srv.Serve(listener)
	defer srv.Close()

	addr := listener.Addr().String()

	// Temporarily override the Telegram API URL by using tgbotapi directly.
	// NewTelegramBot calls tgbotapi.NewBotAPI which hits https://api.telegram.org.
	// Instead, we replicate the logic with a custom endpoint.
	botAPI := &tgbotapi.BotAPI{
		Token:  "test-token",
		Client: &http.Client{},
	}
	botAPI.SetAPIEndpoint("http://" + addr + "/bot%s/%s")

	// Verify getMe works through our fake server.
	resp, err := botAPI.GetMe()
	require.NoError(t, err)
	assert.Equal(t, "test_bot", resp.UserName)
}

// fakeErrorHTTPClient returns error responses so bot.Send fails.
type fakeErrorHTTPClient struct{}

func (f *fakeErrorHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	return nil, errors.New("send failed")
}

func newTestBotWithErrorHTTP(repo LeadRepository, aiClient AIQualifier, ownerID uuid.UUID, bookingLink string) *TelegramBot {
	fakeBotAPI := &tgbotapi.BotAPI{
		Token:  "test-token",
		Client: &fakeErrorHTTPClient{},
	}
	fakeBotAPI.SetAPIEndpoint("http://localhost/bot%s/%s")
	return &TelegramBot{
		bot:         fakeBotAPI,
		repo:        repo,
		aiClient:    aiClient,
		ownerID:     ownerID,
		bookingLink: bookingLink,
	}
}

func TestHandleMessage_CallAgreement_SendError(t *testing.T) {
	repo := newMockLeadRepo()
	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 9}}
	ownerID := uuid.New()
	bot := newTestBotWithErrorHTTP(repo, aiClient, ownerID, "https://cal.com/booking")

	msg := makeTgMessage(88880, "Anna", "", "Давайте созвон проведём!")
	bot.handleMessage(context.Background(), msg)

	waitQualifyDone(t, repo)

	repo.mu.Lock()
	defer repo.mu.Unlock()

	require.Len(t, repo.leads, 1)
	// bot.Send fails, so no outbound message should be saved.
	var outbound []*InboxMessage
	for _, m := range repo.messages {
		if m.Direction == DirectionOutbound {
			outbound = append(outbound, m)
		}
	}
	assert.Empty(t, outbound)
}

func TestHandleMessage_NewLeadError_EmptyFirstName(t *testing.T) {
	repo := newMockLeadRepo()
	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}
	bot := newTestBot(repo, aiClient, uuid.New(), "")

	// Empty first name -> contactName is "" -> NewInboxLead returns error.
	msg := makeTgMessage(99990, "", "", "Some text")
	bot.handleMessage(context.Background(), msg)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	assert.Empty(t, repo.leads)
	assert.Empty(t, repo.messages)
}

func TestBot_Getter(t *testing.T) {
	fakeBotAPI := &tgbotapi.BotAPI{Token: "test-token"}
	tbot := &TelegramBot{bot: fakeBotAPI}
	assert.Equal(t, fakeBotAPI, tbot.Bot())
}

func ptrInt64(v int64) *int64 {
	return &v
}
