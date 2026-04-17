package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutboundStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status OutboundStatus
		want   bool
	}{
		{"draft is valid", OutboundStatusDraft, true},
		{"approved is valid", OutboundStatusApproved, true},
		{"sent is valid", OutboundStatusSent, true},
		{"rejected is valid", OutboundStatusRejected, true},
		{"bounced is valid", OutboundStatusBounced, true},
		{"empty is invalid", OutboundStatus(""), false},
		{"unknown is invalid", OutboundStatus("unknown"), false},
		{"DRAFT uppercase is invalid", OutboundStatus("DRAFT"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.status.IsValid())
		})
	}
}

func TestOutboundStatus_String(t *testing.T) {
	assert.Equal(t, "draft", OutboundStatusDraft.String())
	assert.Equal(t, "approved", OutboundStatusApproved.String())
	assert.Equal(t, "sent", OutboundStatusSent.String())
	assert.Equal(t, "rejected", OutboundStatusRejected.String())
	assert.Equal(t, "bounced", OutboundStatusBounced.String())
}

func TestStepChannel_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		channel StepChannel
		want    bool
	}{
		{"email is valid", StepChannelEmail, true},
		{"telegram is valid", StepChannelTelegram, true},
		{"phone_call is valid", StepChannelPhoneCall, true},
		{"empty is invalid", StepChannel(""), false},
		{"unknown is invalid", StepChannel("sms"), false},
		{"EMAIL uppercase is invalid", StepChannel("EMAIL"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.channel.IsValid())
		})
	}
}

func TestStepChannel_String(t *testing.T) {
	assert.Equal(t, "email", StepChannelEmail.String())
	assert.Equal(t, "telegram", StepChannelTelegram.String())
	assert.Equal(t, "phone_call", StepChannelPhoneCall.String())
}

func TestNewSequence_RejectsInvalidInput(t *testing.T) {
	_, err := NewSequence(uuid.Nil, "Fine Name")
	require.Error(t, err, "zero userID must be rejected")
	_, err = NewSequence(uuid.New(), "")
	require.Error(t, err, "empty name must be rejected")
	_, err = NewSequence(uuid.New(), "   ")
	require.Error(t, err, "blank name must be rejected")
}

func TestSequence_Rename(t *testing.T) {
	s, err := NewSequence(uuid.New(), "Original")
	require.NoError(t, err)
	require.NoError(t, s.Rename("Renamed"))
	assert.Equal(t, "Renamed", s.Name)
	require.Error(t, s.Rename(""), "blank rename must be rejected")
	require.Error(t, s.Rename("\t"), "whitespace rename must be rejected")
}

func TestSequence_ActivateDeactivate(t *testing.T) {
	s, _ := NewSequence(uuid.New(), "x")
	assert.False(t, s.IsActive, "new sequences start inactive")
	s.Activate()
	assert.True(t, s.IsActive)
	s.Deactivate()
	assert.False(t, s.IsActive)
}

func TestNewSequence(t *testing.T) {
	userID := uuid.New()
	name := "Test Sequence"

	s, _ := NewSequence(userID, name)

	assert.NotEqual(t, uuid.Nil, s.ID)
	assert.Equal(t, userID, s.UserID)
	assert.Equal(t, name, s.Name)
	assert.False(t, s.IsActive)
	assert.False(t, s.CreatedAt.IsZero())
}

func TestNewSequenceStep(t *testing.T) {
	sequenceID := uuid.New()
	stepOrder := 2
	delayDays := 3
	channel := StepChannelEmail
	hint := "follow up on intro"

	step := NewSequenceStep(sequenceID, stepOrder, delayDays, channel, hint)

	assert.NotEqual(t, uuid.Nil, step.ID)
	assert.Equal(t, sequenceID, step.SequenceID)
	assert.Equal(t, stepOrder, step.StepOrder)
	assert.Equal(t, delayDays, step.DelayDays)
	assert.Equal(t, channel, step.Channel)
	assert.Equal(t, hint, step.PromptHint)
	assert.False(t, step.CreatedAt.IsZero())
}

func TestOutboundStatus_CanTransitionTo(t *testing.T) {
	legal := map[OutboundStatus][]OutboundStatus{
		OutboundStatusDraft:    {OutboundStatusApproved, OutboundStatusRejected},
		OutboundStatusApproved: {OutboundStatusSent, OutboundStatusRejected, OutboundStatusBounced},
		OutboundStatusSent:     {OutboundStatusBounced},
	}
	// Terminal: rejected, bounced
	for _, term := range []OutboundStatus{OutboundStatusRejected, OutboundStatusBounced} {
		for _, target := range []OutboundStatus{OutboundStatusDraft, OutboundStatusApproved, OutboundStatusSent, OutboundStatusRejected, OutboundStatusBounced} {
			if term.CanTransitionTo(target) {
				t.Errorf("terminal %q must not transition to %q", term, target)
			}
		}
	}
	for from, targets := range legal {
		allowed := map[OutboundStatus]bool{}
		for _, s := range targets {
			allowed[s] = true
		}
		for _, target := range []OutboundStatus{OutboundStatusDraft, OutboundStatusApproved, OutboundStatusSent, OutboundStatusRejected, OutboundStatusBounced} {
			got := from.CanTransitionTo(target)
			want := allowed[target]
			if got != want {
				t.Errorf("%q→%q: got %v, want %v", from, target, got, want)
			}
		}
	}
}

func TestOutboundMessage_TransitionTo_Happy(t *testing.T) {
	m := NewOutboundMessage(uuid.New(), uuid.New(), 1, StepChannelEmail, "hi", time.Now())
	if err := m.TransitionTo(OutboundStatusApproved); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if m.Status != OutboundStatusApproved {
		t.Fatalf("got %q", m.Status)
	}
}

func TestOutboundMessage_TransitionTo_Illegal(t *testing.T) {
	m := NewOutboundMessage(uuid.New(), uuid.New(), 1, StepChannelEmail, "hi", time.Now())
	if err := m.TransitionTo(OutboundStatusSent); err == nil {
		t.Fatal("draft→sent must error (skipped approval)")
	}
	if err := m.TransitionTo("wat"); err == nil {
		t.Fatal("invalid status must error")
	}
}

func TestOutboundMessage_MarkBounced_FromApproved(t *testing.T) {
	m := NewOutboundMessage(uuid.New(), uuid.New(), 1, StepChannelEmail, "hi", time.Now())
	_ = m.TransitionTo(OutboundStatusApproved)
	bouncedAt := time.Now().UTC()
	require.NoError(t, m.MarkBounced(bouncedAt))
	assert.Equal(t, OutboundStatusBounced, m.Status)
	require.NotNil(t, m.BouncedAt)
	assert.Equal(t, bouncedAt, *m.BouncedAt)
}

func TestOutboundMessage_MarkBounced_FromSent(t *testing.T) {
	m := NewOutboundMessage(uuid.New(), uuid.New(), 1, StepChannelEmail, "hi", time.Now())
	_ = m.TransitionTo(OutboundStatusApproved)
	_ = m.MarkSent(time.Now().UTC())
	require.NoError(t, m.MarkBounced(time.Now().UTC()))
	assert.Equal(t, OutboundStatusBounced, m.Status)
	assert.NotNil(t, m.BouncedAt)
}

func TestOutboundMessage_MarkBounced_FromDraft_Rejected(t *testing.T) {
	m := NewOutboundMessage(uuid.New(), uuid.New(), 1, StepChannelEmail, "hi", time.Now())
	// Draft → Bounced must be rejected by the state machine (you can't bounce
	// a message that was never sent or approved).
	require.Error(t, m.MarkBounced(time.Now().UTC()))
	assert.Equal(t, OutboundStatusDraft, m.Status)
	assert.Nil(t, m.BouncedAt)
}

func TestOutboundMessage_MarkBounced_AlreadyBounced_Rejected(t *testing.T) {
	m := NewOutboundMessage(uuid.New(), uuid.New(), 1, StepChannelEmail, "hi", time.Now())
	_ = m.TransitionTo(OutboundStatusApproved)
	_ = m.MarkBounced(time.Now().UTC())
	firstBouncedAt := m.BouncedAt
	// Re-bouncing a terminal-state message must fail; the BouncedAt must
	// not be mutated (domain refuses, doesn't silently overwrite).
	require.Error(t, m.MarkBounced(time.Now().UTC()))
	assert.Equal(t, firstBouncedAt, m.BouncedAt)
}

func TestOutboundMessage_MarkSent(t *testing.T) {
	m := NewOutboundMessage(uuid.New(), uuid.New(), 1, StepChannelEmail, "hi", time.Now())
	_ = m.TransitionTo(OutboundStatusApproved)
	sentAt := time.Now().UTC()
	if err := m.MarkSent(sentAt); err != nil {
		t.Fatalf("mark sent: %v", err)
	}
	if m.Status != OutboundStatusSent || m.SentAt == nil || !m.SentAt.Equal(sentAt) {
		t.Fatalf("unexpected state after MarkSent: status=%q sent_at=%v", m.Status, m.SentAt)
	}

	// Cannot mark a draft as sent directly.
	draft := NewOutboundMessage(uuid.New(), uuid.New(), 1, StepChannelEmail, "hi", time.Now())
	if err := draft.MarkSent(sentAt); err == nil {
		t.Fatal("MarkSent on draft must error")
	}
}

func TestNewOutboundMessage(t *testing.T) {
	prospectID := uuid.New()
	sequenceID := uuid.New()
	stepOrder := 1
	channel := StepChannelTelegram
	body := "Hello, let's connect!"
	scheduledAt := time.Now().UTC().Add(24 * time.Hour)

	msg := NewOutboundMessage(prospectID, sequenceID, stepOrder, channel, body, scheduledAt)

	assert.NotEqual(t, uuid.Nil, msg.ID)
	assert.Equal(t, OutboundStatusDraft, msg.Status)
	assert.Equal(t, prospectID, msg.ProspectID)
	assert.Equal(t, sequenceID, msg.SequenceID)
	assert.Equal(t, stepOrder, msg.StepOrder)
	assert.Equal(t, channel, msg.Channel)
	assert.Equal(t, body, msg.Body)
	assert.Equal(t, scheduledAt, msg.ScheduledAt)
	assert.False(t, msg.CreatedAt.IsZero())
}

// Note: the eligibility predicate that previously lived on ProspectView
// (`CanReceiveSequence`) has been moved back to its sole owner in the
// prospects context. ProspectView now carries a pre-computed
// IsEligibleForSequence boolean populated by the adapter — see
// prospects.domain.Prospect.CanLaunchSequence and its tests.
