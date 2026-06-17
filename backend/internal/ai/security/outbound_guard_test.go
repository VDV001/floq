package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func defaultGuard() *OutboundGuard {
	return NewOutboundGuard(OutboundPolicy{
		AllowedChannels:   []string{"email", "telegram"},
		MassSendThreshold: 100,
		MassSendConfirmed: false,
	})
}

func TestOutboundGuard_AllowsValidEmail(t *testing.T) {
	d := defaultGuard().CheckRecipient("email", "lead@acme.io")
	assert.True(t, d.Allowed)
}

func TestOutboundGuard_RefusesMalformedEmail(t *testing.T) {
	d := defaultGuard().CheckRecipient("email", "not-an-email")
	assert.False(t, d.Allowed)
	assert.NotEmpty(t, d.Reason)
}

func TestOutboundGuard_RefusesEmptyRecipient(t *testing.T) {
	d := defaultGuard().CheckRecipient("email", "")
	assert.False(t, d.Allowed)
}

func TestOutboundGuard_AllowsTelegramTarget(t *testing.T) {
	d := defaultGuard().CheckRecipient("telegram", "@ivan")
	assert.True(t, d.Allowed)
}

func TestOutboundGuard_RefusesEmptyTelegramTarget(t *testing.T) {
	d := defaultGuard().CheckRecipient("telegram", "")
	assert.False(t, d.Allowed)
}

func TestOutboundGuard_RefusesUnknownChannel(t *testing.T) {
	d := defaultGuard().CheckRecipient("sms", "+79123456789")
	assert.False(t, d.Allowed)
	assert.Contains(t, d.Reason, "channel")
}

func TestOutboundGuard_FailsClosedWithNoAllowedChannels(t *testing.T) {
	g := NewOutboundGuard(OutboundPolicy{})
	d := g.CheckRecipient("email", "lead@acme.io")
	assert.False(t, d.Allowed, "no configured channels must fail closed")
}

func TestOutboundGuard_BatchWithinThreshold(t *testing.T) {
	d := defaultGuard().CheckBatch(50)
	assert.True(t, d.Allowed)
}

func TestOutboundGuard_BatchOverThresholdRefusedWithoutConfirmation(t *testing.T) {
	d := defaultGuard().CheckBatch(150)
	assert.False(t, d.Allowed)
	assert.Contains(t, d.Reason, "mass send")
}

func TestOutboundGuard_BatchOverThresholdAllowedWhenConfirmed(t *testing.T) {
	g := NewOutboundGuard(OutboundPolicy{
		AllowedChannels:   []string{"email"},
		MassSendThreshold: 100,
		MassSendConfirmed: true,
	})
	d := g.CheckBatch(150)
	assert.True(t, d.Allowed)
}

func TestOutboundGuard_BatchThresholdDisabledWhenZero(t *testing.T) {
	g := NewOutboundGuard(OutboundPolicy{AllowedChannels: []string{"email"}, MassSendThreshold: 0})
	d := g.CheckBatch(100000)
	assert.True(t, d.Allowed)
}
