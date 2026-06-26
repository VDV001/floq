package inbox

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInboxLead_NormalizesEmailAddress(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"uppercase + whitespace", "  ALICE@Example.COM  ", "alice@example.com"},
		{"already canonical", "alice@example.com", "alice@example.com"},
		{"mixed case", "Alice@Acme.Com", "alice@acme.com"},
		{"tab and newline", "\talice@acme.com\n", "alice@acme.com"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw := c.raw
			lead, err := NewInboxLead(uuid.New(), ChannelEmail, "Alice", "Acme", "hi", nil, &raw)
			require.NoError(t, err)
			require.NotNil(t, lead.EmailAddress)
			assert.Equal(t, c.want, *lead.EmailAddress)
		})
	}
}

func TestNewInboxLead_NilEmailAddressStaysNil(t *testing.T) {
	chatID := int64(12345)
	lead, err := NewInboxLead(uuid.New(), ChannelTelegram, "Alice", "Acme", "hi", &chatID, nil)
	require.NoError(t, err)
	assert.Nil(t, lead.EmailAddress)
}

// TestProcessEmail_LookupDedupsUppercaseEmail asserts that inbound emails
// from the same sender hit the existing-lead branch regardless of case in
// the From header: the lookup must normalize before comparing against the
// canonical address stored on InboxLead.EmailAddress.
func TestProcessEmail_LookupDedupsUppercaseEmail(t *testing.T) {
	repo := newEmailMockLeadRepo()
	prospectRepo := newEmailMockProspectRepo()
	seqRepo := newMockSequenceRepo()
	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}
	ownerID := uuid.New()

	existing := &InboxLead{
		ID:           uuid.New(),
		UserID:       ownerID,
		Channel:      ChannelEmail,
		ContactName:  "Alice",
		FirstMessage: "Earlier message",
		Status:       StatusQualified,
		EmailAddress: ptrString("alice@example.com"),
	}
	repo.existingLeadByEmail["alice@example.com"] = existing

	poller := newTestEmailPoller(repo, prospectRepo, seqRepo, aiClient, ownerID)
	poller.processEmail(context.Background(), "Alice", "  ALICE@Example.COM  ", "Follow-up", nil)

	repo.mu.Lock()
	defer repo.mu.Unlock()

	assert.Empty(t, repo.leads, "uppercased From header must dedup against canonical existing email")
	require.Len(t, repo.messages, 1)
	assert.Equal(t, existing.ID, repo.messages[0].LeadID)
}

// TestHandleMessage_LookupMatchesProspectWithUppercaseUsername asserts the
// prospect lookup canonicalizes the Telegram username before hitting the
// repo, so casing differences in From.UserName don't bypass cross-channel
// dedup with prospects stored in lowercase by NewProspect.
func TestHandleMessage_LookupMatchesProspectWithUppercaseUsername(t *testing.T) {
	repo := newMockLeadRepo()
	prospectRepo := newMockProspectRepo()
	ownerID := uuid.New()

	prospectID := uuid.New()
	srcID := uuid.New()
	prospectRepo.byTgUser["alice_bot"] = &ProspectMatch{
		ID:       prospectID,
		Name:     "Alice Bot",
		Company:  "Acme",
		SourceID: &srcID,
		Status:   "new",
	}

	aiClient := &mockAIQualifier{result: &QualificationResult{Score: 5}}
	bot := newTestBotWithProspects(repo, prospectRepo, aiClient, ownerID, "")

	msg := makeTgMessageWithUsername(42, "Alice", "Bot", "ALICE_BOT", "Привет")
	bot.handleMessage(context.Background(), msg)
	waitQualifyDone(t, repo)

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.leads, 1)
	lead := repo.leads[0]
	assert.Equal(t, "Alice Bot", lead.ContactName, "prospect's name must override From.FirstName when matched")
	assert.Equal(t, "Acme", lead.Company)
	assert.Equal(t, &srcID, lead.SourceID)

	prospectRepo.mu.Lock()
	defer prospectRepo.mu.Unlock()
	require.Len(t, prospectRepo.converted, 1, "matching prospect must trigger auto-conversion")
	assert.Equal(t, prospectID, prospectRepo.converted[0])
}
