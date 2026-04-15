package inbox

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/leads/domain"
	settingsdomain "github.com/daniil/floq/internal/settings/domain"
)

// --- Extended mock for email tests ---

// emailMockLeadRepo extends mockLeadRepo with email lookup support.
type emailMockLeadRepo struct {
	mockLeadRepo
	existingLeadByEmail map[string]*domain.Lead
	getLeadByEmailErr   error
	createLeadErr       error
	createMessageErr    error
}

func newEmailMockLeadRepo() *emailMockLeadRepo {
	return &emailMockLeadRepo{
		mockLeadRepo: mockLeadRepo{
			updatedStatuses:      make(map[uuid.UUID]domain.LeadStatus),
			updatedFirstMessages: make(map[uuid.UUID]string),
			existingLeadByChatID: make(map[int64]*domain.Lead),
			qualifyDone:          make(chan struct{}, 1),
		},
		existingLeadByEmail: make(map[string]*domain.Lead),
	}
}

func (m *emailMockLeadRepo) GetLeadByEmailAddress(_ context.Context, _ uuid.UUID, email string) (*domain.Lead, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getLeadByEmailErr != nil {
		return nil, m.getLeadByEmailErr
	}
	if lead, ok := m.existingLeadByEmail[email]; ok {
		return lead, nil
	}
	return nil, nil
}

func (m *emailMockLeadRepo) CreateLead(_ context.Context, lead *domain.Lead) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createLeadErr != nil {
		return m.createLeadErr
	}
	m.leads = append(m.leads, lead)
	return nil
}

func (m *emailMockLeadRepo) CreateMessage(_ context.Context, msg *domain.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createMessageErr != nil {
		return m.createMessageErr
	}
	m.messages = append(m.messages, msg)
	return nil
}

// --- Mock ProspectRepository with email support ---

type emailMockProspectRepo struct {
	mu        sync.Mutex
	byEmail   map[string]*ProspectMatch
	byTgUser  map[string]*ProspectMatch
	converted []uuid.UUID
}

func newEmailMockProspectRepo() *emailMockProspectRepo {
	return &emailMockProspectRepo{
		byEmail:  make(map[string]*ProspectMatch),
		byTgUser: make(map[string]*ProspectMatch),
	}
}

func (m *emailMockProspectRepo) FindByEmail(_ context.Context, _ uuid.UUID, email string) (*ProspectMatch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.byEmail[email], nil
}

func (m *emailMockProspectRepo) FindByTelegramUsername(_ context.Context, _ uuid.UUID, username string) (*ProspectMatch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.byTgUser[username], nil
}

func (m *emailMockProspectRepo) ConvertToLead(_ context.Context, prospectID, _ uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.converted = append(m.converted, prospectID)
	return nil
}

// --- Mock SequenceRepository ---

type mockSequenceRepo struct {
	mu       sync.Mutex
	markReplied []uuid.UUID
}

func newMockSequenceRepo() *mockSequenceRepo {
	return &mockSequenceRepo{}
}

func (m *mockSequenceRepo) MarkRepliedByProspect(_ context.Context, prospectID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.markReplied = append(m.markReplied, prospectID)
	return nil
}

// --- Helper to build EmailPoller for tests ---

func newTestEmailPoller(repo LeadRepository, prospectRepo ProspectRepository, seqRepo SequenceRepository, aiClient AIQualifier, ownerID uuid.UUID) *EmailPoller {
	return &EmailPoller{
		repo:         repo,
		prospectRepo: prospectRepo,
		seqRepo:      seqRepo,
		aiClient:     aiClient,
		ownerID:      ownerID,
	}
}

// =============================================
// shouldSkipEmail tests
// =============================================

func TestShouldSkipEmail(t *testing.T) {
	tests := []struct {
		name   string
		email  string
		expect bool
	}{
		{"noreply prefix", "noreply@example.com", true},
		{"no-reply prefix", "no-reply@example.com", true},
		{"no_reply prefix", "no_reply@example.com", true},
		{"mailer-daemon prefix", "mailer-daemon@example.com", true},
		{"postmaster prefix", "postmaster@example.com", true},
		{"noreply uppercase", "NOREPLY@example.com", true},
		{"normal email passes", "john@example.com", false},
		{"normal gmail passes", "john.doe@gmail.com", false},
		{"notification at gmail", "notification@gmail.com", true},
		{"newsletter at linkedin", "newsletter@linkedin.com", true},
		{"bounce at google", "bounce@google.com", true},
		{"updates at facebookmail", "updates@facebookmail.com", true},
		{"personal at linkedin passes", "ivan@linkedin.com", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipEmail(tt.email)
			assert.Equal(t, tt.expect, got, "shouldSkipEmail(%q)", tt.email)
		})
	}
}

// =============================================
// extractTextBody tests
// =============================================

func TestExtractTextBody_PlainText(t *testing.T) {
	// A minimal MIME message with text/plain content.
	raw := "Content-Type: text/plain; charset=utf-8\r\n\r\nHello, this is a test email."
	result := extractTextBody([]byte(raw))
	assert.Equal(t, "Hello, this is a test email.", result)
}

func TestExtractTextBody_HTMLFallback(t *testing.T) {
	// Only HTML part — should fall back to it.
	raw := "Content-Type: text/html; charset=utf-8\r\n\r\n<p>Hello HTML</p>"
	result := extractTextBody([]byte(raw))
	assert.Equal(t, "<p>Hello HTML</p>", result)
}

func TestExtractTextBody_MultipartPreferPlain(t *testing.T) {
	raw := "MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/alternative; boundary=boundary42\r\n\r\n" +
		"--boundary42\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n\r\n" +
		"<p>HTML body</p>\r\n" +
		"--boundary42\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n\r\n" +
		"Plain text body\r\n" +
		"--boundary42--\r\n"
	result := extractTextBody([]byte(raw))
	assert.Equal(t, "Plain text body", result)
}

func TestExtractTextBody_EmptyBody(t *testing.T) {
	result := extractTextBody([]byte(""))
	assert.Equal(t, "", result)
}

func TestExtractTextBody_RawFallback(t *testing.T) {
	// Not a valid MIME — falls back to raw text.
	raw := "Just some raw text without headers"
	result := extractTextBody([]byte(raw))
	assert.Equal(t, "Just some raw text without headers", result)
}

// =============================================
// processEmail tests
// =============================================

func TestProcessEmail_NewLead_NoProspect(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := &mockAIQualifier{
		result: &ai.QualificationResult{
			IdentifiedNeed: "Website development",
			Score:          8,
		},
	}
	ownerID := uuid.New()
	poller := newTestEmailPoller(repo, prospectRepo, seqRepo, aiClient, ownerID)

	poller.processEmail(context.Background(), "John Doe", "john@example.com", "I need a website built")

	// Wait for async qualification.
	waitQualifyDone(t, &repo.mockLeadRepo)

	repo.mu.Lock()
	defer repo.mu.Unlock()

	// A new lead should be created.
	require.Len(t, repo.leads, 1)
	lead := repo.leads[0]
	assert.Equal(t, ownerID, lead.UserID)
	assert.Equal(t, domain.ChannelEmail, lead.Channel)
	assert.Equal(t, "John Doe", lead.ContactName)
	assert.Equal(t, "I need a website built", lead.FirstMessage)
	require.NotNil(t, lead.EmailAddress)
	assert.Equal(t, "john@example.com", *lead.EmailAddress)
	assert.Nil(t, lead.SourceID)

	// Inbound message should be created.
	require.Len(t, repo.messages, 1)
	assert.Equal(t, lead.ID, repo.messages[0].LeadID)
	assert.Equal(t, domain.DirectionInbound, repo.messages[0].Direction)
	assert.Equal(t, "I need a website built", repo.messages[0].Body)

	// Qualification should run.
	require.Len(t, repo.qualifications, 1)
	assert.Equal(t, lead.ID, repo.qualifications[0].LeadID)
	assert.Equal(t, 8, repo.qualifications[0].Score)
	assert.Equal(t, domain.StatusQualified, repo.updatedStatuses[lead.ID])

	// No prospect conversion.
	prospectRepo.mu.Lock()
	defer prospectRepo.mu.Unlock()
	assert.Empty(t, prospectRepo.converted)

	// No sequence repo calls.
	seqRepo.mu.Lock()
	defer seqRepo.mu.Unlock()
	assert.Empty(t, seqRepo.markReplied)
}

func TestProcessEmail_NewLead_WithProspectMatch(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := &mockAIQualifier{
		result: &ai.QualificationResult{Score: 6},
	}
	ownerID := uuid.New()

	prospectID := uuid.New()
	srcID := uuid.New()
	prospectRepo.byEmail["prospect@company.com"] = &ProspectMatch{
		ID:       prospectID,
		Name:     "Prospect Name",
		Company:  "ProspectCo",
		SourceID: &srcID,
		Status:   "new",
	}

	poller := newTestEmailPoller(repo, prospectRepo, seqRepo, aiClient, ownerID)

	// Use email as fromName to test name override from prospect.
	poller.processEmail(context.Background(), "prospect@company.com", "prospect@company.com", "Interested in your service")

	waitQualifyDone(t, &repo.mockLeadRepo)

	repo.mu.Lock()
	defer repo.mu.Unlock()

	require.Len(t, repo.leads, 1)
	lead := repo.leads[0]
	// Name should be overridden by prospect name (since fromName == fromEmail).
	assert.Equal(t, "Prospect Name", lead.ContactName)
	assert.Equal(t, "ProspectCo", lead.Company)
	// SourceID should be copied from prospect.
	assert.Equal(t, &srcID, lead.SourceID)

	// Prospect should be converted.
	prospectRepo.mu.Lock()
	defer prospectRepo.mu.Unlock()
	require.Len(t, prospectRepo.converted, 1)
	assert.Equal(t, prospectID, prospectRepo.converted[0])

	// Sequence should be marked as replied.
	seqRepo.mu.Lock()
	defer seqRepo.mu.Unlock()
	require.Len(t, seqRepo.markReplied, 1)
	assert.Equal(t, prospectID, seqRepo.markReplied[0])
}

func TestProcessEmail_ExistingLead_AddsMessage(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := &mockAIQualifier{
		result: &ai.QualificationResult{Score: 5},
	}
	ownerID := uuid.New()

	existingLead := &domain.Lead{
		ID:           uuid.New(),
		UserID:       ownerID,
		Channel:      domain.ChannelEmail,
		ContactName:  "Existing User",
		FirstMessage: "Previous message",
		Status:       domain.StatusQualified,
		EmailAddress: ptrString("existing@example.com"),
	}
	repo.existingLeadByEmail["existing@example.com"] = existingLead

	poller := newTestEmailPoller(repo, prospectRepo, seqRepo, aiClient, ownerID)

	poller.processEmail(context.Background(), "Existing User", "existing@example.com", "Follow-up message")

	// For existing leads, no async qualification — give a short moment just in case.
	// Actually processEmail only qualifies new leads, so no waitQualifyDone needed.

	repo.mu.Lock()
	defer repo.mu.Unlock()

	// No new lead should be created.
	assert.Empty(t, repo.leads)

	// A message should be created for the existing lead.
	require.Len(t, repo.messages, 1)
	assert.Equal(t, existingLead.ID, repo.messages[0].LeadID)
	assert.Equal(t, domain.DirectionInbound, repo.messages[0].Direction)
	assert.Equal(t, "Follow-up message", repo.messages[0].Body)

	// No qualification for existing leads.
	assert.Empty(t, repo.qualifications)

	// No prospect conversion.
	prospectRepo.mu.Lock()
	defer prospectRepo.mu.Unlock()
	assert.Empty(t, prospectRepo.converted)
}

func TestProcessEmail_ProspectAlreadyConverted_SkipsConversion(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := &mockAIQualifier{
		result: &ai.QualificationResult{Score: 5},
	}
	ownerID := uuid.New()

	prospectRepo.byEmail["converted@example.com"] = &ProspectMatch{
		ID:     uuid.New(),
		Name:   "Already Converted",
		Status: "converted",
	}

	poller := newTestEmailPoller(repo, prospectRepo, seqRepo, aiClient, ownerID)

	poller.processEmail(context.Background(), "Someone", "converted@example.com", "Hello again")

	waitQualifyDone(t, &repo.mockLeadRepo)

	repo.mu.Lock()
	defer repo.mu.Unlock()

	// Lead should be created but with original name (not prospect name, since prospect is converted).
	require.Len(t, repo.leads, 1)
	assert.Equal(t, "Someone", repo.leads[0].ContactName)
	assert.Nil(t, repo.leads[0].SourceID)

	// No prospect conversion.
	prospectRepo.mu.Lock()
	defer prospectRepo.mu.Unlock()
	assert.Empty(t, prospectRepo.converted)

	// No sequence marking.
	seqRepo.mu.Lock()
	defer seqRepo.mu.Unlock()
	assert.Empty(t, seqRepo.markReplied)
}

func TestProcessEmail_NewLead_ProspectNameNotOverriddenWhenFromNameDiffers(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := &mockAIQualifier{
		result: &ai.QualificationResult{Score: 5},
	}
	ownerID := uuid.New()

	prospectRepo.byEmail["john@company.com"] = &ProspectMatch{
		ID:      uuid.New(),
		Name:    "Prospect John",
		Company: "JohnCo",
		Status:  "new",
	}

	poller := newTestEmailPoller(repo, prospectRepo, seqRepo, aiClient, ownerID)

	// fromName != fromEmail, so prospect name should NOT override.
	poller.processEmail(context.Background(), "John D.", "john@company.com", "Hello")

	waitQualifyDone(t, &repo.mockLeadRepo)

	repo.mu.Lock()
	defer repo.mu.Unlock()

	require.Len(t, repo.leads, 1)
	// fromName is "John D." which differs from fromEmail, so it stays.
	assert.Equal(t, "John D.", repo.leads[0].ContactName)
	// But company should still be set from prospect.
	assert.Equal(t, "JohnCo", repo.leads[0].Company)
}

func TestProcessEmail_GetLeadByEmailError(t *testing.T) {
	repo := newEmailMockLeadRepo()
	repo.getLeadByEmailErr = errors.New("db error")
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := &mockAIQualifier{result: &ai.QualificationResult{Score: 5}}
	ownerID := uuid.New()

	poller := newTestEmailPoller(repo, prospectRepo, seqRepo, aiClient, ownerID)

	poller.processEmail(context.Background(), "Test", "test@example.com", "Hello")

	repo.mu.Lock()
	defer repo.mu.Unlock()

	// Should return early, no leads or messages created.
	assert.Empty(t, repo.leads)
	assert.Empty(t, repo.messages)
}

func TestProcessEmail_CreateLeadError(t *testing.T) {
	repo := newEmailMockLeadRepo()
	repo.createLeadErr = errors.New("insert failed")
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := &mockAIQualifier{result: &ai.QualificationResult{Score: 5}}
	ownerID := uuid.New()

	poller := newTestEmailPoller(repo, prospectRepo, seqRepo, aiClient, ownerID)

	poller.processEmail(context.Background(), "Test", "test@example.com", "Hello")

	repo.mu.Lock()
	defer repo.mu.Unlock()

	// CreateLead fails, so no message should be created either.
	assert.Empty(t, repo.messages)
}

func TestProcessEmail_CreateMessageError(t *testing.T) {
	repo := newEmailMockLeadRepo()
	repo.createMessageErr = errors.New("message insert failed")
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := &mockAIQualifier{result: &ai.QualificationResult{Score: 5}}
	ownerID := uuid.New()

	// Existing lead — skips qualification, only creates message.
	existingLead := &domain.Lead{
		ID:           uuid.New(),
		UserID:       ownerID,
		Channel:      domain.ChannelEmail,
		ContactName:  "Existing",
		FirstMessage: "Old",
		Status:       domain.StatusQualified,
		EmailAddress: ptrString("err@example.com"),
	}
	repo.existingLeadByEmail["err@example.com"] = existingLead

	poller := newTestEmailPoller(repo, prospectRepo, seqRepo, aiClient, ownerID)

	poller.processEmail(context.Background(), "Existing", "err@example.com", "Follow-up")

	repo.mu.Lock()
	defer repo.mu.Unlock()

	// CreateMessage fails, so should return early. No qualification.
	assert.Empty(t, repo.qualifications)
}

func TestProcessEmail_AIQualificationError(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := &errorAIQualifier{}
	ownerID := uuid.New()

	poller := newTestEmailPoller(repo, prospectRepo, seqRepo, aiClient, ownerID)

	poller.processEmail(context.Background(), "Test", "test@example.com", "Hello")

	// Give goroutine time to finish (returns early on error).
	time.Sleep(200 * time.Millisecond)

	repo.mu.Lock()
	defer repo.mu.Unlock()

	require.Len(t, repo.leads, 1)
	require.Len(t, repo.messages, 1)
	assert.Empty(t, repo.qualifications)
	assert.Empty(t, repo.updatedStatuses)
}

func TestProcessEmail_ProspectConvertError(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := &errorConvertEmailProspectRepo{
		emailMockProspectRepo: emailMockProspectRepo{
			byEmail: map[string]*ProspectMatch{
				"conv-err@example.com": {
					ID:     uuid.New(),
					Name:   "Conv Error",
					Status: "new",
				},
			},
			byTgUser: make(map[string]*ProspectMatch),
		},
	}
	seqRepo := newMockSequenceRepo()
	aiClient := &mockAIQualifier{result: &ai.QualificationResult{Score: 5}}
	ownerID := uuid.New()

	poller := newTestEmailPoller(repo, prospectRepo, seqRepo, aiClient, ownerID)

	poller.processEmail(context.Background(), "conv-err@example.com", "conv-err@example.com", "Hello")

	waitQualifyDone(t, &repo.mockLeadRepo)

	repo.mu.Lock()
	defer repo.mu.Unlock()

	// Lead created despite convert error.
	require.Len(t, repo.leads, 1)
}

// --- Error ConvertToLead prospect repo for email ---

type errorConvertEmailProspectRepo struct {
	emailMockProspectRepo
}

func (m *errorConvertEmailProspectRepo) ConvertToLead(_ context.Context, _, _ uuid.UUID) error {
	return errors.New("convert failed")
}

// --- Upsert/Status error repos for email ---

type emailUpsertErrorRepo struct {
	emailMockLeadRepo
}

func (m *emailUpsertErrorRepo) UpsertQualification(_ context.Context, _ *domain.Qualification) error {
	select {
	case m.qualifyDone <- struct{}{}:
	default:
	}
	return errors.New("upsert failed")
}

func TestProcessEmail_NewLeadError_EmptyName(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := &mockAIQualifier{result: &ai.QualificationResult{Score: 5}}
	ownerID := uuid.New()

	poller := newTestEmailPoller(repo, prospectRepo, seqRepo, aiClient, ownerID)

	// Empty fromName triggers NewLead error (contactName required).
	poller.processEmail(context.Background(), "", "empty@example.com", "Hello")

	repo.mu.Lock()
	defer repo.mu.Unlock()
	assert.Empty(t, repo.leads)
	assert.Empty(t, repo.messages)
}

func TestProcessEmail_UpsertQualificationError(t *testing.T) {
	inner := newEmailMockLeadRepo()
	repo := &emailUpsertErrorRepo{emailMockLeadRepo: *inner}
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := &mockAIQualifier{result: &ai.QualificationResult{Score: 5}}
	ownerID := uuid.New()

	poller := newTestEmailPoller(repo, prospectRepo, seqRepo, aiClient, ownerID)

	poller.processEmail(context.Background(), "Test", "test@example.com", "Hello")

	waitQualifyDone(t, &repo.mockLeadRepo)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.leads, 1)
	assert.Empty(t, repo.updatedStatuses)
}

type emailStatusErrorRepo struct {
	emailMockLeadRepo
}

func (m *emailStatusErrorRepo) UpdateLeadStatus(_ context.Context, _ uuid.UUID, _ domain.LeadStatus) error {
	select {
	case m.qualifyDone <- struct{}{}:
	default:
	}
	return errors.New("status update failed")
}

func TestProcessEmail_UpdateLeadStatusError(t *testing.T) {
	inner := newEmailMockLeadRepo()
	repo := &emailStatusErrorRepo{emailMockLeadRepo: *inner}
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := &mockAIQualifier{result: &ai.QualificationResult{Score: 5}}
	ownerID := uuid.New()

	poller := newTestEmailPoller(repo, prospectRepo, seqRepo, aiClient, ownerID)

	poller.processEmail(context.Background(), "Test", "test@example.com", "Hello")

	waitQualifyDone(t, &repo.mockLeadRepo)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.leads, 1)
	require.Len(t, repo.qualifications, 1)
}

func TestNewEmailPoller(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := &mockAIQualifier{result: &ai.QualificationResult{}}
	ownerID := uuid.New()

	poller := NewEmailPoller(
		&mockConfigStore{},
		ownerID,
		"imap.example.com", "993", "user@example.com", "pass",
		repo, prospectRepo, seqRepo, aiClient,
	)

	require.NotNil(t, poller)
	assert.Equal(t, ownerID, poller.ownerID)
	assert.Equal(t, "imap.example.com", poller.fallbackHost)
	assert.Equal(t, "993", poller.fallbackPort)
	assert.Equal(t, "user@example.com", poller.fallbackUser)
	assert.Equal(t, "pass", poller.fallbackPassword)
}

// --- Mock ConfigStore ---

type mockConfigStore struct {
	cfg *settingsdomain.UserConfig
	err error
}

func (m *mockConfigStore) GetConfig(_ context.Context, _ uuid.UUID) (*settingsdomain.UserConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.cfg != nil {
		return m.cfg, nil
	}
	return &settingsdomain.UserConfig{}, nil
}

func TestResolveConfig_Fallback(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := &mockAIQualifier{result: &ai.QualificationResult{}}

	store := &mockConfigStore{err: errors.New("no config")}
	poller := &EmailPoller{
		store:            store,
		repo:             repo,
		prospectRepo:     prospectRepo,
		seqRepo:          seqRepo,
		aiClient:         aiClient,
		ownerID:          uuid.New(),
		fallbackHost:     "fallback.host",
		fallbackPort:     "993",
		fallbackUser:     "fallback@user.com",
		fallbackPassword: "fallbackpass",
	}

	host, port, user, password := poller.resolveConfig(context.Background())
	assert.Equal(t, "fallback.host", host)
	assert.Equal(t, "993", port)
	assert.Equal(t, "fallback@user.com", user)
	assert.Equal(t, "fallbackpass", password)
}

func TestResolveConfig_WithUserConfig(t *testing.T) {
	store := &mockConfigStore{
		cfg: &settingsdomain.UserConfig{
			IMAPHost:     "custom.host",
			IMAPPort:     "465",
			IMAPUser:     "custom@user.com",
			IMAPPassword: "custompass",
		},
	}
	poller := &EmailPoller{
		store:            store,
		ownerID:          uuid.New(),
		fallbackHost:     "fallback.host",
		fallbackPort:     "993",
		fallbackUser:     "fallback@user.com",
		fallbackPassword: "fallbackpass",
	}

	host, port, user, password := poller.resolveConfig(context.Background())
	assert.Equal(t, "custom.host", host)
	assert.Equal(t, "465", port)
	assert.Equal(t, "custom@user.com", user)
	assert.Equal(t, "custompass", password)
}

func ptrString(s string) *string {
	return &s
}

// --- Start / poll tests ---

func TestEmailPoller_Start_CancelledContext(t *testing.T) {
	// Start should return when context is cancelled.
	// With empty credentials, poll returns immediately, then we cancel context.
	poller := &EmailPoller{
		store: &mockConfigStore{err: errors.New("no config")},
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		poller.Start(ctx)
		close(done)
	}()

	// Give Start a moment to run poll, then cancel
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// OK — Start returned
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}

func TestEmailPoller_Poll_EmptyCredentials(t *testing.T) {
	// When host/user/password are empty, poll should return immediately without error.
	poller := &EmailPoller{
		store: &mockConfigStore{err: errors.New("no config")},
	}
	// Should not panic or block
	poller.poll(context.Background())
}

func TestEmailPoller_Poll_ConnectionError(t *testing.T) {
	// With a real but unreachable host, poll should log error and return.
	poller := &EmailPoller{
		store:            &mockConfigStore{err: errors.New("no config")},
		fallbackHost:     "127.0.0.1",
		fallbackPort:     "19993",
		fallbackUser:     "user@test.com",
		fallbackPassword: "pass",
	}
	// Should not panic or block — just logs connection error
	poller.poll(context.Background())
}
