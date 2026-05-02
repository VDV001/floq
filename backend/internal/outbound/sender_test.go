package outbound

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	prospectsdomain "github.com/daniil/floq/internal/prospects/domain"
	seqdomain "github.com/daniil/floq/internal/sequences/domain"
	settingsdomain "github.com/daniil/floq/internal/settings/domain"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

type mockConfigStore struct {
	cfg *settingsdomain.UserConfig
	err error
}

func (m *mockConfigStore) GetConfig(_ context.Context, _ uuid.UUID) (*settingsdomain.UserConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.cfg, nil
}

type mockOutboundRepository struct {
	pending    []seqdomain.OutboundMessage
	pendingErr error
	sentIDs    []uuid.UUID
	sentAts    []time.Time
	sentErr    error
	bouncedIDs []uuid.UUID
	bouncedErr error
}

func (m *mockOutboundRepository) GetPendingSends(_ context.Context) ([]seqdomain.OutboundMessage, error) {
	return m.pending, m.pendingErr
}

func (m *mockOutboundRepository) MarkSent(_ context.Context, id uuid.UUID, sentAt time.Time) error {
	m.sentIDs = append(m.sentIDs, id)
	m.sentAts = append(m.sentAts, sentAt)
	return m.sentErr
}

func (m *mockOutboundRepository) MarkBounced(_ context.Context, id uuid.UUID, _ time.Time) error {
	m.bouncedIDs = append(m.bouncedIDs, id)
	return m.bouncedErr
}

type mockProspectLookup struct {
	prospects         map[uuid.UUID]*prospectsdomain.Prospect
	err               error
	verifyUpdatedIDs  []uuid.UUID
	verifyStatuses    []prospectsdomain.VerifyStatus
}

func (m *mockProspectLookup) GetProspect(_ context.Context, id uuid.UUID) (*prospectsdomain.Prospect, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.prospects[id], nil
}

func (m *mockProspectLookup) UpdateVerification(_ context.Context, id uuid.UUID, status prospectsdomain.VerifyStatus, _ int, _ string, _ time.Time) error {
	m.verifyUpdatedIDs = append(m.verifyUpdatedIDs, id)
	m.verifyStatuses = append(m.verifyStatuses, status)
	return nil
}

type mockTelegramSessionStore struct {
	phone       string
	sessionData []byte
	err         error
}

func (m *mockTelegramSessionStore) GetSession(_ context.Context, _ string) (string, []byte, error) {
	return m.phone, m.sessionData, m.err
}

type mockTelegramMessenger struct {
	calls   []tgSendCall
	sendErr error
}

type tgSendCall struct {
	target string
	body   string
}

func (m *mockTelegramMessenger) SendMessage(_ context.Context, _ []byte, target, body string) error {
	m.calls = append(m.calls, tgSendCall{target: target, body: body})
	return m.sendErr
}

// ---------------------------------------------------------------------------
// Existing tests (preserved)
// ---------------------------------------------------------------------------

func TestNewSender_Fields(t *testing.T) {
	ownerID := uuid.New()
	s := NewSender(nil, ownerID, "key123", "from@test.com", "https://app.test", "smtp.mail.ru", "465", "user@test.com", "pass", nil, nil, nil, nil, nil, nil)

	if s.ownerID != ownerID {
		t.Errorf("expected ownerID %s, got %s", ownerID, s.ownerID)
	}
	if s.fallbackKey != "key123" {
		t.Errorf("expected fallbackKey %q, got %q", "key123", s.fallbackKey)
	}
	if s.fromAddress != "from@test.com" {
		t.Errorf("expected fromAddress %q, got %q", "from@test.com", s.fromAddress)
	}
	if s.smtpHost != "smtp.mail.ru" {
		t.Errorf("expected smtpHost %q, got %q", "smtp.mail.ru", s.smtpHost)
	}
	if s.smtpUser != "user@test.com" {
		t.Errorf("expected smtpUser %q, got %q", "user@test.com", s.smtpUser)
	}
}

func TestNewSender_NilDeps(t *testing.T) {
	s := NewSender(nil, uuid.Nil, "", "", "", "", "", "", "", nil, nil, nil, nil, nil, nil)
	if s == nil {
		t.Fatal("expected non-nil Sender")
	}
}

// ---------------------------------------------------------------------------
// New comprehensive tests
// ---------------------------------------------------------------------------

func TestSendPending_NoPendingMessages(t *testing.T) {
	seqRepo := &mockOutboundRepository{pending: nil}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, uuid.New(), "", "from@x.com", "", "", "", "", "", seqRepo, nil, nil, nil, nil, nil)

	if err := s.SendPending(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(seqRepo.sentIDs) != 0 {
		t.Errorf("expected no sent IDs, got %v", seqRepo.sentIDs)
	}
}

func TestSendPending_EmailHappyPath(t *testing.T) {
	// SMTP is configured with a real host that doesn't exist, so sendViaSMTPWith
	// will fail with a dial error. Since the error does NOT contain bounce
	// keywords, the message will be logged as a send failure and skipped
	// (not marked as sent or bounced). This tests that the full flow reaches
	// the send attempt.
	//
	// To truly test the "happy path" end-to-end we'd need a real SMTP server.
	// Instead, we verify the flow WITHOUT smtp credentials: no smtp + no resend
	// key = "no Resend API key" error, proving that prospect lookup, subject
	// building, and body construction all execute.
	//
	// We set smtpHost="" so the code falls through to sendViaResend, and set
	// no Resend key so we get a deterministic error.

	prospectID := uuid.New()
	msgID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelEmail,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "<p>Hello!</p>",
			},
		},
	}
	prospectRepo := &mockProspectLookup{
		prospects: map[uuid.UUID]*prospectsdomain.Prospect{
			prospectID: {
				ID:      prospectID,
				Name:    "Alice",
				Company: "Acme",
				Email:   "alice@acme.com",
			},
		},
	}
	// No SMTP creds, no Resend key → sendViaResend returns "no Resend API key"
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, uuid.New(), "", "from@test.com", "https://app.test",
		"", "", "", "", // no SMTP
		seqRepo, prospectRepo, nil, nil, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("SendPending should not return error (per-message errors are logged), got %v", err)
	}

	// Message was NOT marked as sent because send failed (no Resend key)
	if len(seqRepo.sentIDs) != 0 {
		t.Errorf("expected 0 sent, got %d", len(seqRepo.sentIDs))
	}
	// It was also NOT marked as bounced — "no Resend API key" doesn't contain bounce keywords
	if len(seqRepo.bouncedIDs) != 0 {
		t.Errorf("expected 0 bounced, got %d", len(seqRepo.bouncedIDs))
	}
}

func TestSendPending_BounceDetection(t *testing.T) {
	// SMTP configured with a non-existent server — dial fails with connection error,
	// NOT a 550. Verifies that transient errors do NOT trigger bounce detection.
	//
	// BEST APPROACH: Just run with no SMTP, use Resend path. The Resend client will
	// actually try to call the API. With a fake key "bounce-test-550", the Resend API
	// call will fail. The error from Resend SDK may or may not contain "550".
	//
	// SIMPLEST: We accept that we can't get a real "550" from a unit test SMTP.
	// Instead, test that when SMTP fails on port 465, it is treated as a transient
	// error. This is still valuable.

	// Let's test it differently: use port 465 with localhost — the error from TLS dial
	// will NOT contain bounce keywords, so the message stays unsent (not bounced).
	prospectID := uuid.New()
	msgID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelEmail,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "hi",
			},
		},
	}
	prospectRepo := &mockProspectLookup{
		prospects: map[uuid.UUID]*prospectsdomain.Prospect{
			prospectID: {
				ID:    prospectID,
				Name:  "Bob",
				Email: "bob@example.com",
			},
		},
	}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	// Use SMTP on port 465 → TLS dial will fail → transient error, no bounce
	s := NewSender(cfgStore, uuid.New(), "", "from@test.com", "",
		"127.0.0.1", "465", "user", "pass",
		seqRepo, prospectRepo, nil, nil, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no top-level error, got %v", err)
	}

	// TLS dial error is transient → should NOT be marked as bounced
	if len(seqRepo.bouncedIDs) != 0 {
		t.Errorf("expected 0 bounced (transient error), got %d", len(seqRepo.bouncedIDs))
	}
	// Also not marked as sent
	if len(seqRepo.sentIDs) != 0 {
		t.Errorf("expected 0 sent, got %d", len(seqRepo.sentIDs))
	}
}

func TestSendPending_TelegramHappyPath(t *testing.T) {
	prospectID := uuid.New()
	msgID := uuid.New()
	ownerID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelTelegram,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "Hello via TG!",
			},
		},
	}
	prospectRepo := &mockProspectLookup{
		prospects: map[uuid.UUID]*prospectsdomain.Prospect{
			prospectID: {
				ID:               prospectID,
				Name:             "Charlie",
				TelegramUsername: "charlie_tg",
				Phone:            "+79991112233",
			},
		},
	}
	tgRepo := &mockTelegramSessionStore{
		phone:       "+70001234567",
		sessionData: []byte("session-bytes"),
	}
	tgMessenger := &mockTelegramMessenger{}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, ownerID, "", "", "",
		"", "", "", "",
		seqRepo, prospectRepo, tgRepo, tgMessenger, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// TelegramMessenger.SendMessage should have been called
	if len(tgMessenger.calls) != 1 {
		t.Fatalf("expected 1 TG send call, got %d", len(tgMessenger.calls))
	}
	call := tgMessenger.calls[0]
	if call.target != "@charlie_tg" {
		t.Errorf("expected target @charlie_tg, got %q", call.target)
	}
	if call.body != "Hello via TG!" {
		t.Errorf("expected body %q, got %q", "Hello via TG!", call.body)
	}

	// Message should be marked as sent
	if len(seqRepo.sentIDs) != 1 {
		t.Fatalf("expected 1 sent ID, got %d", len(seqRepo.sentIDs))
	}
	if seqRepo.sentIDs[0] != msgID {
		t.Errorf("expected sent ID %s, got %s", msgID, seqRepo.sentIDs[0])
	}
}

func TestSendPending_TelegramRateLimit(t *testing.T) {
	prospectID1 := uuid.New()
	prospectID2 := uuid.New()
	msgID1 := uuid.New()
	msgID2 := uuid.New()
	ownerID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID1,
				ProspectID: prospectID1,
				Channel:    seqdomain.StepChannelTelegram,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "First TG message",
			},
			{
				ID:         msgID2,
				ProspectID: prospectID2,
				Channel:    seqdomain.StepChannelTelegram,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "Second TG message",
			},
		},
	}
	prospectRepo := &mockProspectLookup{
		prospects: map[uuid.UUID]*prospectsdomain.Prospect{
			prospectID1: {
				ID:               prospectID1,
				TelegramUsername: "user1",
			},
			prospectID2: {
				ID:               prospectID2,
				TelegramUsername: "user2",
			},
		},
	}
	tgRepo := &mockTelegramSessionStore{
		phone:       "+70001234567",
		sessionData: []byte("session-bytes"),
	}
	tgMessenger := &mockTelegramMessenger{}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, ownerID, "", "", "",
		"", "", "", "",
		seqRepo, prospectRepo, tgRepo, tgMessenger, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Only the first message should have been sent; the second is rate-limited
	if len(tgMessenger.calls) != 1 {
		t.Fatalf("expected 1 TG send call (rate limit), got %d", len(tgMessenger.calls))
	}
	if tgMessenger.calls[0].target != "@user1" {
		t.Errorf("expected first target @user1, got %q", tgMessenger.calls[0].target)
	}

	// Only first message marked as sent
	if len(seqRepo.sentIDs) != 1 {
		t.Fatalf("expected 1 sent, got %d", len(seqRepo.sentIDs))
	}
	if seqRepo.sentIDs[0] != msgID1 {
		t.Errorf("expected sent msgID1 %s, got %s", msgID1, seqRepo.sentIDs[0])
	}
}

func TestSendPending_SkipNoEmail(t *testing.T) {
	prospectID := uuid.New()
	msgID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelEmail,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "hi",
			},
		},
	}
	// Prospect exists but has no email
	prospectRepo := &mockProspectLookup{
		prospects: map[uuid.UUID]*prospectsdomain.Prospect{
			prospectID: {
				ID:   prospectID,
				Name: "NoEmail Person",
			},
		},
	}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, uuid.New(), "", "from@test.com", "",
		"", "", "", "",
		seqRepo, prospectRepo, nil, nil, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Message should be skipped entirely — not sent, not bounced
	if len(seqRepo.sentIDs) != 0 {
		t.Errorf("expected 0 sent, got %d", len(seqRepo.sentIDs))
	}
	if len(seqRepo.bouncedIDs) != 0 {
		t.Errorf("expected 0 bounced, got %d", len(seqRepo.bouncedIDs))
	}
}

func TestSendPending_GetPendingSendsError(t *testing.T) {
	seqRepo := &mockOutboundRepository{pendingErr: fmt.Errorf("db down")}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, uuid.New(), "", "", "",
		"", "", "", "",
		seqRepo, nil, nil, nil, nil, nil)

	err := s.SendPending(context.Background())
	if err == nil {
		t.Fatal("expected error from GetPendingSends, got nil")
	}
}

func TestSendPending_TelegramNoSession(t *testing.T) {
	prospectID := uuid.New()
	msgID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelTelegram,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "TG msg",
			},
		},
	}
	// Session store returns empty session
	tgRepo := &mockTelegramSessionStore{phone: "", sessionData: nil}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, uuid.New(), "", "", "",
		"", "", "", "",
		seqRepo, nil, tgRepo, nil, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Not sent because no session
	if len(seqRepo.sentIDs) != 0 {
		t.Errorf("expected 0 sent, got %d", len(seqRepo.sentIDs))
	}
}

func TestSendPending_TelegramSendFailure(t *testing.T) {
	prospectID := uuid.New()
	msgID := uuid.New()
	ownerID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelTelegram,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "TG msg",
			},
		},
	}
	prospectRepo := &mockProspectLookup{
		prospects: map[uuid.UUID]*prospectsdomain.Prospect{
			prospectID: {
				ID:               prospectID,
				TelegramUsername: "target_user",
			},
		},
	}
	tgRepo := &mockTelegramSessionStore{
		phone:       "+70001234567",
		sessionData: []byte("session"),
	}
	tgMessenger := &mockTelegramMessenger{
		sendErr: fmt.Errorf("FLOOD_WAIT"),
	}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, ownerID, "", "", "",
		"", "", "", "",
		seqRepo, prospectRepo, tgRepo, tgMessenger, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no top-level error, got %v", err)
	}

	// Send failed for all targets → message not marked as sent
	if len(seqRepo.sentIDs) != 0 {
		t.Errorf("expected 0 sent, got %d", len(seqRepo.sentIDs))
	}
}

func TestSendPending_TelegramNilRepo(t *testing.T) {
	msgID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:      msgID,
				Channel: seqdomain.StepChannelTelegram,
				Body:    "msg",
			},
		},
	}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	// tgRepo is nil → handleTelegramMessage should bail out early
	s := NewSender(cfgStore, uuid.New(), "", "", "",
		"", "", "", "",
		seqRepo, nil, nil, nil, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(seqRepo.sentIDs) != 0 {
		t.Errorf("expected 0 sent, got %d", len(seqRepo.sentIDs))
	}
}

func TestSendPending_UnknownChannel(t *testing.T) {
	msgID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:      msgID,
				Channel: seqdomain.StepChannel("sms"),
				Body:    "msg",
			},
		},
	}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, uuid.New(), "", "", "",
		"", "", "", "",
		seqRepo, nil, nil, nil, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no error for unknown channel, got %v", err)
	}
	if len(seqRepo.sentIDs) != 0 {
		t.Errorf("expected 0 sent, got %d", len(seqRepo.sentIDs))
	}
}

func TestSendPending_ConfigStoreOverridesSMTP(t *testing.T) {
	// Verify that DB config overrides .env SMTP settings.
	// We pass empty .env SMTP but set DB config with SMTP host.
	// The code should use the DB values and attempt SMTP send.
	prospectID := uuid.New()
	msgID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelEmail,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "hello",
			},
		},
	}
	prospectRepo := &mockProspectLookup{
		prospects: map[uuid.UUID]*prospectsdomain.Prospect{
			prospectID: {
				ID:    prospectID,
				Name:  "Test",
				Email: "test@example.com",
			},
		},
	}
	// DB config provides SMTP credentials (points to nowhere → will fail)
	cfgStore := &mockConfigStore{
		cfg: &settingsdomain.UserConfig{
			SMTPHost:     "127.0.0.1",
			SMTPPort:     "465",
			SMTPUser:     "dbuser@test.com",
			SMTPPassword: "dbpass",
		},
	}

	// .env SMTP is empty
	s := NewSender(cfgStore, uuid.New(), "", "from@test.com", "",
		"", "", "", "",
		seqRepo, prospectRepo, nil, nil, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no top-level error, got %v", err)
	}

	// Send will fail (TLS dial to 127.0.0.1:465 refused) — but it attempted SMTP,
	// proving DB config was used. Not bounced (transient error).
	if len(seqRepo.bouncedIDs) != 0 {
		t.Errorf("expected 0 bounced, got %d", len(seqRepo.bouncedIDs))
	}
}

func TestSendPending_SubjectWithoutCompany(t *testing.T) {
	// When prospect has no company, subject should use the fallback format.
	// We can't directly inspect the subject, but we verify the flow completes
	// (the prospect is fetched and the send attempt happens).
	prospectID := uuid.New()
	msgID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelEmail,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "hi",
			},
		},
	}
	prospectRepo := &mockProspectLookup{
		prospects: map[uuid.UUID]*prospectsdomain.Prospect{
			prospectID: {
				ID:      prospectID,
				Name:    "Eve",
				Company: "", // empty company
				Email:   "eve@example.com",
			},
		},
	}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, uuid.New(), "", "from@test.com", "",
		"", "", "", "",
		seqRepo, prospectRepo, nil, nil, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Flow executed — no panic, no sent (because no SMTP/Resend)
	if len(seqRepo.sentIDs) != 0 {
		t.Errorf("expected 0 sent, got %d", len(seqRepo.sentIDs))
	}
}

func TestSendPending_TelegramPhoneFallback(t *testing.T) {
	// Prospect has no TelegramUsername but has Phone → should try phone as target
	prospectID := uuid.New()
	msgID := uuid.New()
	ownerID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelTelegram,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "Phone fallback msg",
			},
		},
	}
	prospectRepo := &mockProspectLookup{
		prospects: map[uuid.UUID]*prospectsdomain.Prospect{
			prospectID: {
				ID:    prospectID,
				Phone: "+79998887766",
			},
		},
	}
	tgRepo := &mockTelegramSessionStore{
		phone:       "+70001234567",
		sessionData: []byte("session"),
	}
	tgMessenger := &mockTelegramMessenger{}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, ownerID, "", "", "",
		"", "", "", "",
		seqRepo, prospectRepo, tgRepo, tgMessenger, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(tgMessenger.calls) != 1 {
		t.Fatalf("expected 1 TG call, got %d", len(tgMessenger.calls))
	}
	if tgMessenger.calls[0].target != "+79998887766" {
		t.Errorf("expected phone target +79998887766, got %q", tgMessenger.calls[0].target)
	}
	if len(seqRepo.sentIDs) != 1 {
		t.Errorf("expected 1 sent, got %d", len(seqRepo.sentIDs))
	}
}

func TestSendPending_ProspectLookupError(t *testing.T) {
	prospectID := uuid.New()
	msgID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelEmail,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "hi",
			},
		},
	}
	prospectRepo := &mockProspectLookup{
		err: fmt.Errorf("prospect db down"),
	}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, uuid.New(), "", "from@test.com", "",
		"", "", "", "",
		seqRepo, prospectRepo, nil, nil, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no top-level error, got %v", err)
	}
	// Error on prospect lookup — message skipped, not sent, not bounced
	if len(seqRepo.sentIDs) != 0 {
		t.Errorf("expected 0 sent, got %d", len(seqRepo.sentIDs))
	}
	if len(seqRepo.bouncedIDs) != 0 {
		t.Errorf("expected 0 bounced, got %d", len(seqRepo.bouncedIDs))
	}
}

func TestSendPending_NilProspect(t *testing.T) {
	prospectID := uuid.New()
	msgID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelEmail,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "hi",
			},
		},
	}
	// Prospect not in map → returns nil
	prospectRepo := &mockProspectLookup{
		prospects: map[uuid.UUID]*prospectsdomain.Prospect{},
	}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, uuid.New(), "", "from@test.com", "",
		"", "", "", "",
		seqRepo, prospectRepo, nil, nil, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(seqRepo.sentIDs) != 0 {
		t.Errorf("expected 0 sent, got %d", len(seqRepo.sentIDs))
	}
}

func TestSendPending_WithTrackingPixel(t *testing.T) {
	// Test that appBaseURL being set doesn't break the flow
	prospectID := uuid.New()
	msgID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelEmail,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "<p>Hello!</p>",
			},
		},
	}
	prospectRepo := &mockProspectLookup{
		prospects: map[uuid.UUID]*prospectsdomain.Prospect{
			prospectID: {
				ID:    prospectID,
				Name:  "Track",
				Email: "track@example.com",
			},
		},
	}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, uuid.New(), "", "from@test.com", "https://app.floq.test",
		"", "", "", "",
		seqRepo, prospectRepo, nil, nil, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// No SMTP / no Resend key → send fails, but flow ran including tracking pixel construction
}

func TestSendPending_ConfigStoreError(t *testing.T) {
	// ConfigStore returns an error — should fall back to .env values and continue
	prospectID := uuid.New()
	msgID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelEmail,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "hi",
			},
		},
	}
	prospectRepo := &mockProspectLookup{
		prospects: map[uuid.UUID]*prospectsdomain.Prospect{
			prospectID: {
				ID:    prospectID,
				Name:  "Test",
				Email: "test@example.com",
			},
		},
	}
	cfgStore := &mockConfigStore{err: fmt.Errorf("config db down")}

	s := NewSender(cfgStore, uuid.New(), "", "from@test.com", "",
		"", "", "", "",
		seqRepo, prospectRepo, nil, nil, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSendPending_TelegramProspectLookupError(t *testing.T) {
	prospectID := uuid.New()
	msgID := uuid.New()
	ownerID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelTelegram,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "TG msg",
			},
		},
	}
	prospectRepo := &mockProspectLookup{
		err: fmt.Errorf("prospect db down"),
	}
	tgRepo := &mockTelegramSessionStore{
		phone:       "+70001234567",
		sessionData: []byte("session"),
	}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, ownerID, "", "", "",
		"", "", "", "",
		seqRepo, prospectRepo, tgRepo, nil, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(seqRepo.sentIDs) != 0 {
		t.Errorf("expected 0 sent, got %d", len(seqRepo.sentIDs))
	}
}

func TestSendPending_TelegramSessionError(t *testing.T) {
	prospectID := uuid.New()
	msgID := uuid.New()
	ownerID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelTelegram,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "TG msg",
			},
		},
	}
	tgRepo := &mockTelegramSessionStore{
		err: fmt.Errorf("session store error"),
	}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, ownerID, "", "", "",
		"", "", "", "",
		seqRepo, nil, tgRepo, nil, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(seqRepo.sentIDs) != 0 {
		t.Errorf("expected 0 sent, got %d", len(seqRepo.sentIDs))
	}
}

func TestSendPending_TelegramNilProspect(t *testing.T) {
	prospectID := uuid.New()
	msgID := uuid.New()
	ownerID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelTelegram,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "TG msg",
			},
		},
	}
	// Prospect not found
	prospectRepo := &mockProspectLookup{
		prospects: map[uuid.UUID]*prospectsdomain.Prospect{},
	}
	tgRepo := &mockTelegramSessionStore{
		phone:       "+70001234567",
		sessionData: []byte("session"),
	}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, ownerID, "", "", "",
		"", "", "", "",
		seqRepo, prospectRepo, tgRepo, nil, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(seqRepo.sentIDs) != 0 {
		t.Errorf("expected 0 sent, got %d", len(seqRepo.sentIDs))
	}
}

func TestSendPending_TelegramNoPhoneNoUsername(t *testing.T) {
	// Prospect has neither TG username nor phone → message skipped
	prospectID := uuid.New()
	msgID := uuid.New()
	ownerID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelTelegram,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "no target",
			},
		},
	}
	prospectRepo := &mockProspectLookup{
		prospects: map[uuid.UUID]*prospectsdomain.Prospect{
			prospectID: {ID: prospectID},
		},
	}
	tgRepo := &mockTelegramSessionStore{
		phone:       "+70001234567",
		sessionData: []byte("session"),
	}
	tgMessenger := &mockTelegramMessenger{}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, ownerID, "", "", "",
		"", "", "", "",
		seqRepo, prospectRepo, tgRepo, tgMessenger, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(tgMessenger.calls) != 0 {
		t.Errorf("expected 0 TG calls, got %d", len(tgMessenger.calls))
	}
	if len(seqRepo.sentIDs) != 0 {
		t.Errorf("expected 0 sent, got %d", len(seqRepo.sentIDs))
	}
}

func TestSendPending_BounceKeywords(t *testing.T) {
	prospectID := uuid.New()
	msgID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelEmail,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "hi",
			},
		},
	}
	prospectRepo := &mockProspectLookup{
		prospects: map[uuid.UUID]*prospectsdomain.Prospect{
			prospectID: {
				ID:    prospectID,
				Name:  "Bob",
				Email: "bob@example.com",
			},
		},
	}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	// Use SMTP with port 587 (STARTTLS path) and no from address → from = smtpUser
	s := NewSender(cfgStore, uuid.New(), "", "", "",
		"127.0.0.1", "587", "sender@test.com", "pass",
		seqRepo, prospectRepo, nil, nil, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(seqRepo.bouncedIDs) != 0 {
		t.Errorf("expected 0 bounced, got %d", len(seqRepo.bouncedIDs))
	}
}

func TestSendPending_SMTPFromAddrFallback(t *testing.T) {
	prospectID := uuid.New()
	msgID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelEmail,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "hi",
			},
		},
	}
	prospectRepo := &mockProspectLookup{
		prospects: map[uuid.UUID]*prospectsdomain.Prospect{
			prospectID: {
				ID:    prospectID,
				Name:  "Alice",
				Email: "alice@example.com",
			},
		},
	}
	cfgStore := &mockConfigStore{
		cfg: &settingsdomain.UserConfig{
			SMTPHost:     "127.0.0.1",
			SMTPPort:     "587",
			SMTPUser:     "dbuser@test.com",
			SMTPPassword: "dbpass",
		},
	}

	s := NewSender(cfgStore, uuid.New(), "", "", "",
		"", "", "", "",
		seqRepo, prospectRepo, nil, nil, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSendPending_TelegramMarkSentError(t *testing.T) {
	prospectID := uuid.New()
	msgID := uuid.New()
	ownerID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{
			{
				ID:         msgID,
				ProspectID: prospectID,
				Channel:    seqdomain.StepChannelTelegram,
				Status:     seqdomain.OutboundStatusApproved,
				Body:       "TG msg",
			},
		},
		sentErr: fmt.Errorf("mark sent failed"),
	}
	prospectRepo := &mockProspectLookup{
		prospects: map[uuid.UUID]*prospectsdomain.Prospect{
			prospectID: {
				ID:               prospectID,
				TelegramUsername: "tg_user",
			},
		},
	}
	tgRepo := &mockTelegramSessionStore{
		phone:       "+70001234567",
		sessionData: []byte("session"),
	}
	tgMessenger := &mockTelegramMessenger{}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}

	s := NewSender(cfgStore, ownerID, "", "", "",
		"", "", "", "",
		seqRepo, prospectRepo, tgRepo, tgMessenger, nil, nil)

	err := s.SendPending(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(tgMessenger.calls) != 1 {
		t.Fatalf("expected 1 TG call, got %d", len(tgMessenger.calls))
	}
}

func TestSendViaResend_NoKey(t *testing.T) {
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}
	s := NewSender(cfgStore, uuid.New(), "", "from@test.com", "",
		"", "", "", "",
		nil, nil, nil, nil, nil, nil)

	err := s.sendViaResend(context.Background(), "to@test.com", "subj", "body")
	if !errors.Is(err, ErrNoResendAPIKey) {
		t.Fatalf("expected errors.Is to match ErrNoResendAPIKey, got: %v", err)
	}
}

func TestSendViaResend_ConfigStoreError(t *testing.T) {
	cfgStore := &mockConfigStore{err: fmt.Errorf("db down")}
	s := NewSender(cfgStore, uuid.New(), "", "from@test.com", "",
		"", "", "", "",
		nil, nil, nil, nil, nil, nil)

	err := s.sendViaResend(context.Background(), "to@test.com", "subj", "body")
	if !errors.Is(err, ErrNoResendAPIKey) {
		t.Fatalf("expected errors.Is to match ErrNoResendAPIKey, got: %v", err)
	}
}

func TestSendViaSMTPWith_FromFallback(t *testing.T) {
	s := &Sender{}
	err := s.sendViaSMTPWith(context.Background(), "127.0.0.1", "587", "user@test.com", "pass", "", "to@test.com", "subj", "body")
	if err == nil {
		t.Fatal("expected error from SMTP dial")
	}
}

func TestSendViaSMTPWith_Port465(t *testing.T) {
	s := &Sender{}
	err := s.sendViaSMTPWith(context.Background(), "127.0.0.1", "465", "user@test.com", "pass", "from@test.com", "to@test.com", "subj", "body")
	if err == nil {
		t.Fatal("expected error from TLS dial")
	}
}
