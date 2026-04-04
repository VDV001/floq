package domain

import (
	"testing"

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
