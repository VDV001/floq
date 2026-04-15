package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
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

func TestNewSequence(t *testing.T) {
	userID := uuid.New()
	name := "Test Sequence"

	s := NewSequence(userID, name)

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

func TestProspectView_CanReceiveSequence(t *testing.T) {
	tests := []struct {
		name   string
		view   ProspectView
		expect bool
	}{
		{
			name:   "status converted returns false",
			view:   ProspectView{Status: ProspectStatusConverted, VerifyStatus: "valid"},
			expect: false,
		},
		{
			name:   "status opted_out returns false",
			view:   ProspectView{Status: ProspectStatusOptedOut, VerifyStatus: "valid"},
			expect: false,
		},
		{
			name:   "status in_sequence returns false",
			view:   ProspectView{Status: ProspectStatusInSequence, VerifyStatus: "valid"},
			expect: false,
		},
		{
			name:   "verify_status invalid returns false",
			view:   ProspectView{Status: "new", VerifyStatus: VerifyStatusInvalid},
			expect: false,
		},
		{
			name:   "verify_status not_checked with email returns false",
			view:   ProspectView{Status: "new", VerifyStatus: VerifyStatusNotChecked, Email: "test@example.com"},
			expect: false,
		},
		{
			name:   "verify_status not_checked without email returns true",
			view:   ProspectView{Status: "new", VerifyStatus: VerifyStatusNotChecked, Email: ""},
			expect: true,
		},
		{
			name:   "verify_status valid with status new returns true",
			view:   ProspectView{Status: "new", VerifyStatus: "valid", Email: "test@example.com"},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, tt.view.CanReceiveSequence())
		})
	}
}
