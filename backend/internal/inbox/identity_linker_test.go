package inbox

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingIdentityLinker captures every invocation of LinkLeadToIdentity
// so tests can assert what payload the inbound flow forwards. A non-zero
// returnErr forces the linker to fail; tests use this to verify graceful
// degradation (lead must still land even when identity wiring breaks).
type recordingIdentityLinker struct {
	mu          sync.Mutex
	invocations []linkLeadInvocation
	returnErr   error
}

type linkLeadInvocation struct {
	UserID, LeadID                 uuid.UUID
	Email, Phone, TelegramUsername string
}

func (l *recordingIdentityLinker) LinkLeadToIdentity(_ context.Context, userID, leadID uuid.UUID, email, phone, tg string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.invocations = append(l.invocations, linkLeadInvocation{
		UserID:           userID,
		LeadID:           leadID,
		Email:            email,
		Phone:            phone,
		TelegramUsername: tg,
	})
	return l.returnErr
}

func (l *recordingIdentityLinker) takeInvocations() []linkLeadInvocation {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]linkLeadInvocation, len(l.invocations))
	copy(out, l.invocations)
	return out
}

func TestProcessEmail_CallsIdentityLinker_WithNormalizedEmail(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	linker := &recordingIdentityLinker{}
	ownerID := uuid.New()

	poller := NewEmailPoller(nil, ownerID, "", "", "", "", repo, prospectRepo, seqRepo, nil,
		WithIdentityLinker(linker))

	poller.processEmail(context.Background(), "Alice", "  ALICE@Example.COM  ", "Hi", nil)

	invs := linker.takeInvocations()
	require.Len(t, invs, 1, "linker must be called exactly once for a new lead")
	got := invs[0]
	assert.Equal(t, ownerID, got.UserID)
	assert.Equal(t, "alice@example.com", got.Email, "linker must receive the canonical email")
	assert.Empty(t, got.Phone)
	assert.Empty(t, got.TelegramUsername)

	require.Len(t, repo.mockLeadRepo.leads, 1)
	assert.Equal(t, repo.mockLeadRepo.leads[0].ID, got.LeadID, "leadID must match the freshly created lead")
}

func TestProcessEmail_LinkerError_DoesNotBreakInboundFlow(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	linker := &recordingIdentityLinker{returnErr: errors.New("identity backend down")}

	poller := NewEmailPoller(nil, uuid.New(), "", "", "", "", repo, prospectRepo, seqRepo, nil,
		WithIdentityLinker(linker))

	poller.processEmail(context.Background(), "Alice", "alice@example.com", "Hi", nil)

	require.Len(t, repo.mockLeadRepo.leads, 1, "lead must be created even when linker fails")
	require.Len(t, repo.mockLeadRepo.messages, 1, "message must still land")
}

func TestProcessEmail_NoLinker_NoOp(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()

	poller := NewEmailPoller(nil, uuid.New(), "", "", "", "", repo, prospectRepo, seqRepo, nil)

	poller.processEmail(context.Background(), "Alice", "alice@example.com", "Hi", nil)

	require.Len(t, repo.mockLeadRepo.leads, 1, "no linker wired = lead still created")
}

func TestProcessEmail_ExistingLead_DoesNotCallLinker(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	linker := &recordingIdentityLinker{}
	ownerID := uuid.New()

	existing := &InboxLead{
		ID:           uuid.New(),
		UserID:       ownerID,
		Channel:      ChannelEmail,
		ContactName:  "Alice",
		FirstMessage: "Earlier",
		Status:       StatusQualified,
		EmailAddress: ptrString("alice@example.com"),
	}
	repo.existingLeadByEmail["alice@example.com"] = existing

	poller := NewEmailPoller(nil, ownerID, "", "", "", "", repo, prospectRepo, seqRepo, nil,
		WithIdentityLinker(linker))

	poller.processEmail(context.Background(), "Alice", "alice@example.com", "Reply", nil)

	assert.Empty(t, linker.takeInvocations(), "linker is only triggered on new lead creation")
}

func TestHandleMessage_CallsIdentityLinker_WithNormalizedUsername(t *testing.T) {
	repo := newMockLeadRepo()
	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}
	linker := &recordingIdentityLinker{}
	ownerID := uuid.New()
	bot := newTestBot(repo, aiClient, ownerID, "")
	bot.identityLinker = linker

	msg := makeTgMessageWithUsername(42, "Alice", "Bot", "ALICE_BOT", "Hi")
	bot.handleMessage(context.Background(), msg)

	invs := linker.takeInvocations()
	require.Len(t, invs, 1)
	got := invs[0]
	assert.Equal(t, ownerID, got.UserID)
	assert.Empty(t, got.Email)
	assert.Empty(t, got.Phone)
	assert.Equal(t, "alice_bot", got.TelegramUsername, "linker must receive the canonical username")
}

func TestHandleMessage_EmptyUsername_SkipsLinker(t *testing.T) {
	repo := newMockLeadRepo()
	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}
	linker := &recordingIdentityLinker{}
	bot := newTestBot(repo, aiClient, uuid.New(), "")
	bot.identityLinker = linker

	// No UserName on the message — linker has nothing to resolve.
	msg := makeTgMessage(43, "Alice", "Bot", "Hello")
	bot.handleMessage(context.Background(), msg)

	assert.Empty(t, linker.takeInvocations(), "linker must be skipped when no identifier is available")
}

func TestHandleMessage_LinkerError_DoesNotBreakInboundFlow(t *testing.T) {
	repo := newMockLeadRepo()
	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}
	linker := &recordingIdentityLinker{returnErr: errors.New("identity backend down")}
	bot := newTestBot(repo, aiClient, uuid.New(), "")
	bot.identityLinker = linker

	msg := makeTgMessageWithUsername(44, "Alice", "Bot", "alice_bot", "Hi")
	bot.handleMessage(context.Background(), msg)

	require.Len(t, repo.leads, 1, "lead must land even when linker fails")
}

func TestHandleMessage_ExistingLead_DoesNotCallLinker(t *testing.T) {
	repo := newMockLeadRepo()
	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}
	linker := &recordingIdentityLinker{}
	ownerID := uuid.New()

	chatID := int64(45)
	repo.existingLeadByChatID[chatID] = &InboxLead{
		ID:           uuid.New(),
		UserID:       ownerID,
		Channel:      ChannelTelegram,
		ContactName:  "Alice",
		FirstMessage: "Earlier",
		Status:       StatusNew,
	}

	bot := newTestBot(repo, aiClient, ownerID, "")
	bot.identityLinker = linker

	msg := makeTgMessageWithUsername(chatID, "Alice", "Bot", "alice_bot", "Reply with more text than the existing first_message threshold of twenty chars")
	bot.handleMessage(context.Background(), msg)

	assert.Empty(t, linker.takeInvocations(), "existing lead path must not re-trigger identity linking")
}
