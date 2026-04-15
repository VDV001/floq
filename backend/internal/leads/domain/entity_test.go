package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLeadStatus_IsValid(t *testing.T) {
	valid := []LeadStatus{StatusNew, StatusQualified, StatusClosed, StatusWon}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("expected %q to be valid", s)
		}
	}

	invalid := []LeadStatus{"", "pending", "lost", "archived"}
	for _, s := range invalid {
		if s.IsValid() {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func TestLeadStatus_String(t *testing.T) {
	tests := []struct {
		status LeadStatus
		want   string
	}{
		{StatusNew, "new"},
		{StatusQualified, "qualified"},
		{StatusClosed, "closed"},
		{StatusWon, "won"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("LeadStatus(%q).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestLeadStatus_CanTransitionTo(t *testing.T) {
	allowed := []struct {
		from, to LeadStatus
	}{
		{StatusNew, StatusQualified},
		{StatusNew, StatusClosed},
		{StatusQualified, StatusClosed},
		{StatusQualified, StatusWon},
		{StatusWon, StatusClosed},
	}
	for _, tt := range allowed {
		if !tt.from.CanTransitionTo(tt.to) {
			t.Errorf("expected transition %s -> %s to be allowed", tt.from, tt.to)
		}
	}

	disallowed := []struct {
		from, to LeadStatus
	}{
		{StatusNew, StatusWon},
		{StatusNew, StatusNew},
		{StatusQualified, StatusNew},
		{StatusQualified, StatusQualified},
		{StatusClosed, StatusNew},
		{StatusClosed, StatusQualified},
		{StatusClosed, StatusWon},
		{StatusClosed, StatusClosed},
		{StatusWon, StatusNew},
		{StatusWon, StatusQualified},
		{StatusWon, StatusWon},
	}
	for _, tt := range disallowed {
		if tt.from.CanTransitionTo(tt.to) {
			t.Errorf("expected transition %s -> %s to be disallowed", tt.from, tt.to)
		}
	}
}

func TestChannel_IsValid(t *testing.T) {
	valid := []Channel{ChannelTelegram, ChannelEmail}
	for _, c := range valid {
		if !c.IsValid() {
			t.Errorf("expected %q to be valid", c)
		}
	}

	invalid := []Channel{"", "sms", "whatsapp", "slack"}
	for _, c := range invalid {
		if c.IsValid() {
			t.Errorf("expected %q to be invalid", c)
		}
	}
}

func TestNewLead(t *testing.T) {
	userID := uuid.New()
	chatID := int64(123)
	lead, err := NewLead(userID, ChannelTelegram, "Ivan", "Acme", "Hello", &chatID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if lead.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if lead.UserID != userID {
		t.Errorf("expected userID %s, got %s", userID, lead.UserID)
	}
	if lead.Status != StatusNew {
		t.Errorf("expected status new, got %s", lead.Status)
	}
	if lead.Channel != ChannelTelegram {
		t.Errorf("expected channel telegram, got %s", lead.Channel)
	}
	if lead.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestLead_TransitionTo_Valid(t *testing.T) {
	lead := &Lead{Status: StatusNew}
	if err := lead.TransitionTo(StatusQualified); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lead.Status != StatusQualified {
		t.Errorf("expected qualified, got %s", lead.Status)
	}
}

func TestLead_TransitionTo_InvalidTarget(t *testing.T) {
	lead := &Lead{Status: StatusNew}
	if err := lead.TransitionTo(LeadStatus("bogus")); err == nil {
		t.Error("expected error for invalid target status")
	}
}

func TestLead_TransitionTo_DisallowedTransition(t *testing.T) {
	lead := &Lead{Status: StatusNew}
	if err := lead.TransitionTo(StatusWon); err == nil {
		t.Error("expected error for disallowed transition new -> won")
	}
	if lead.Status != StatusNew {
		t.Error("status should not change on failed transition")
	}
}

func TestNewMessage(t *testing.T) {
	leadID := uuid.New()
	msg := NewMessage(leadID, DirectionOutbound, "hello")

	if msg.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if msg.LeadID != leadID {
		t.Error("wrong leadID")
	}
	if msg.Direction != DirectionOutbound {
		t.Error("wrong direction")
	}
	if msg.SentAt.IsZero() {
		t.Error("expected non-zero SentAt")
	}
}

func TestDetectCallAgreement_Positive(t *testing.T) {
	positives := []string{
		"давайте созвонимся",
		"Да, давайте обсудим",
		"готов к звонку завтра",
		"Когда удобно?",
	}
	for _, text := range positives {
		assert.True(t, DetectCallAgreement(text), "expected true for %q", text)
	}
}

func TestDetectCallAgreement_Negative(t *testing.T) {
	negatives := []string{
		"нет спасибо",
		"не интересно",
		"привет",
		"",
	}
	for _, text := range negatives {
		assert.False(t, DetectCallAgreement(text), "expected false for %q", text)
	}
}

func TestNewLead_InvalidChannel(t *testing.T) {
	userID := uuid.New()
	_, err := NewLead(userID, Channel("whatsapp"), "Ivan", "Acme", "Hello", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid channel")
}

func TestNewLead_EmptyContactName(t *testing.T) {
	userID := uuid.New()
	_, err := NewLead(userID, ChannelTelegram, "", "Acme", "Hello", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contact name")
}

func TestNewQualification_HappyPath(t *testing.T) {
	leadID := uuid.New()
	q := NewQualification(leadID, "CRM integration", "$10k", "Q3 2026", 85, "strong fit", "schedule demo", "openai")

	assert.NotEqual(t, uuid.Nil, q.ID)
	assert.Equal(t, leadID, q.LeadID)
	assert.Equal(t, "CRM integration", q.IdentifiedNeed)
	assert.Equal(t, "$10k", q.EstimatedBudget)
	assert.Equal(t, "Q3 2026", q.Deadline)
	assert.Equal(t, 85, q.Score)
	assert.Equal(t, "strong fit", q.ScoreReason)
	assert.Equal(t, "schedule demo", q.RecommendedAction)
	assert.Equal(t, "openai", q.ProviderUsed)
	assert.False(t, q.GeneratedAt.IsZero())
}

func TestNewDraft_HappyPath(t *testing.T) {
	leadID := uuid.New()
	d := NewDraft(leadID, "Hello, let's discuss your project")

	assert.NotEqual(t, uuid.Nil, d.ID)
	assert.Equal(t, leadID, d.LeadID)
	assert.Equal(t, "Hello, let's discuss your project", d.Body)
	assert.False(t, d.CreatedAt.IsZero())
}
