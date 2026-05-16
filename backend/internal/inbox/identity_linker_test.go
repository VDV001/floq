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
	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}
	linker := &recordingIdentityLinker{}
	ownerID := uuid.New()

	poller := NewEmailPoller(nil, ownerID, "", "", "", "", repo, prospectRepo, seqRepo, aiClient, nil,
		WithIdentityLinker(linker))

	poller.processEmail(context.Background(), "Alice", "  ALICE@Example.COM  ", "Hi", nil)
	waitQualifyDone(t, &repo.mockLeadRepo)

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
	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}
	linker := &recordingIdentityLinker{returnErr: errors.New("identity backend down")}

	poller := NewEmailPoller(nil, uuid.New(), "", "", "", "", repo, prospectRepo, seqRepo, aiClient, nil,
		WithIdentityLinker(linker))

	poller.processEmail(context.Background(), "Alice", "alice@example.com", "Hi", nil)
	waitQualifyDone(t, &repo.mockLeadRepo)

	require.Len(t, repo.mockLeadRepo.leads, 1, "lead must be created even when linker fails")
	require.Len(t, repo.mockLeadRepo.messages, 1, "message must still land")
}

func TestProcessEmail_NoLinker_NoOp(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}

	poller := NewEmailPoller(nil, uuid.New(), "", "", "", "", repo, prospectRepo, seqRepo, aiClient, nil)

	poller.processEmail(context.Background(), "Alice", "alice@example.com", "Hi", nil)
	waitQualifyDone(t, &repo.mockLeadRepo)

	require.Len(t, repo.mockLeadRepo.leads, 1, "no linker wired = lead still created")
}

func TestProcessEmail_ExistingLead_DoesNotCallLinker(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}
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

	poller := NewEmailPoller(nil, ownerID, "", "", "", "", repo, prospectRepo, seqRepo, aiClient, nil,
		WithIdentityLinker(linker))

	poller.processEmail(context.Background(), "Alice", "alice@example.com", "Reply", nil)

	assert.Empty(t, linker.takeInvocations(), "linker is only triggered on new lead creation")
}
