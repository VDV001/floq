package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLead_InheritsSourceFrom(t *testing.T) {
	leadSource := uuid.New()
	prospectSource := uuid.New()

	tests := []struct {
		name             string
		leadSourceID     *uuid.UUID
		prospectSourceID *uuid.UUID
		wantSource       *uuid.UUID
		wantChanged      bool
	}{
		{"lead has source — kept; prospect source ignored", &leadSource, &prospectSource, &leadSource, false},
		{"lead has source — kept; prospect has none", &leadSource, nil, &leadSource, false},
		{"lead has no source — inherits from prospect", nil, &prospectSource, &prospectSource, true},
		{"both empty — stays nil, not changed", nil, nil, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lead := &Lead{SourceID: tt.leadSourceID}
			gotSource, gotChanged := lead.InheritsSourceFrom(tt.prospectSourceID)
			assert.Equal(t, tt.wantSource, gotSource)
			assert.Equal(t, tt.wantChanged, gotChanged)
		})
	}
}

func TestLead_SetSource(t *testing.T) {
	newSource := uuid.New()
	lead := &Lead{}
	before := lead.UpdatedAt
	lead.SetSource(&newSource)
	assert.Equal(t, &newSource, lead.SourceID)
	assert.True(t, lead.UpdatedAt.After(before), "SetSource should bump updated_at")

	// nil clears
	lead.SetSource(nil)
	assert.Nil(t, lead.SourceID)
}

func TestLead_OnOutboundSent_QualifiedAdvances(t *testing.T) {
	lead := &Lead{Status: StatusQualified}
	changed := lead.OnOutboundSent()
	assert.True(t, changed)
	assert.Equal(t, StatusInConversation, lead.Status)
}

func TestLead_OnOutboundSent_NonQualifiedNoChange(t *testing.T) {
	for _, s := range []LeadStatus{StatusNew, StatusInConversation, StatusFollowup, StatusClosed, StatusWon} {
		lead := &Lead{Status: s}
		changed := lead.OnOutboundSent()
		assert.False(t, changed, "status %q must not transition on outbound send", s)
		assert.Equal(t, s, lead.Status)
	}
}

func TestSuggestionConfidence_IsValid(t *testing.T) {
	valid := []SuggestionConfidence{ConfidenceHigh, ConfidenceMedium, ConfidenceLow}
	for _, c := range valid {
		if !c.IsValid() {
			t.Errorf("expected %q to be valid", c)
		}
	}

	invalid := []SuggestionConfidence{"", "unknown", "HIGH"}
	for _, c := range invalid {
		if c.IsValid() {
			t.Errorf("expected %q to be invalid", c)
		}
	}
}

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
	d, err := NewDraft(leadID, "Hello, let's discuss your project")
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, d.ID)
	assert.Equal(t, leadID, d.LeadID)
	assert.Equal(t, "Hello, let's discuss your project", d.Body)
	assert.False(t, d.CreatedAt.IsZero())
	assert.False(t, d.IsEmpty())
}

func TestNewDraft_RejectsEmptyBody(t *testing.T) {
	_, err := NewDraft(uuid.New(), "")
	require.Error(t, err)
}

func TestNewReminder_Invariants(t *testing.T) {
	_, err := NewReminder(uuid.Nil, "msg")
	require.Error(t, err, "zero leadID must be rejected")
	_, err = NewReminder(uuid.New(), "")
	require.Error(t, err, "empty message must be rejected")

	r, err := NewReminder(uuid.New(), "lead went quiet for 3 days")
	require.NoError(t, err)
	assert.False(t, r.Dismissed)
	assert.NotEqual(t, uuid.Nil, r.ID)
}

func TestReminder_Dismiss_Idempotent(t *testing.T) {
	r, _ := NewReminder(uuid.New(), "x")
	r.Dismiss()
	assert.True(t, r.Dismissed)
	r.Dismiss() // idempotent — double-click must not error or flip back
	assert.True(t, r.Dismissed)
}

func TestRehydrateQualification_PreservesIdentityClampsScore(t *testing.T) {
	id := uuid.New()
	leadID := uuid.New()
	generatedAt := time.Now().UTC().Add(-1 * time.Hour)

	// Score above 100 must be clamped by the factory.
	q := RehydrateQualification(id, leadID, "n", "b", "d", 150, "r", "a", "p", generatedAt)
	assert.Equal(t, id, q.ID)
	assert.Equal(t, leadID, q.LeadID)
	assert.Equal(t, generatedAt, q.GeneratedAt)
	assert.Equal(t, 100, q.Score)

	// Score below 0.
	q = RehydrateQualification(id, leadID, "n", "b", "d", -5, "r", "a", "p", generatedAt)
	assert.Equal(t, 0, q.Score)

	// In-range passes through.
	q = RehydrateQualification(id, leadID, "n", "b", "d", 55, "r", "a", "p", generatedAt)
	assert.Equal(t, 55, q.Score)
	assert.True(t, q.IsWarm())
}

func TestQualification_ScoreBands(t *testing.T) {
	cases := []struct {
		name      string
		score     int
		wantHot   bool
		wantWarm  bool
		wantClamp int // expected score after clamping
	}{
		{"hot (>=80)", 85, true, false, 85},
		{"hot boundary", 80, true, false, 80},
		{"warm (50-79)", 65, false, true, 65},
		{"warm upper boundary", 79, false, true, 79},
		{"warm lower boundary", 50, false, true, 50},
		{"cold (<50)", 20, false, false, 20},
		{"clamp negative", -10, false, false, 0},
		{"clamp over 100", 150, true, false, 100},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := NewQualification(uuid.New(), "n", "b", "d", c.score, "r", "a", "p")
			assert.Equal(t, c.wantClamp, q.Score)
			assert.Equal(t, c.wantHot, q.IsHot())
			assert.Equal(t, c.wantWarm, q.IsWarm())
		})
	}
}
