package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var consentAt = time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)

func TestNewConsent_Obtained(t *testing.T) {
	c, err := NewConsent(ConsentStatusObtained, "inbound_reply", consentAt)
	require.NoError(t, err)
	assert.Equal(t, ConsentStatusObtained, c.Status)
	assert.Equal(t, "inbound_reply", c.Source)
	assert.Equal(t, consentAt, c.Timestamp)
}

func TestNewConsent_NoneNeedsNoSource(t *testing.T) {
	c, err := NewConsent(ConsentStatusNone, "", time.Time{})
	require.NoError(t, err)
	assert.Equal(t, ConsentStatusNone, c.Status)
	assert.Empty(t, c.Source)
}

func TestNewConsent_InvalidStatus(t *testing.T) {
	_, err := NewConsent(ConsentStatus("bogus"), "x", consentAt)
	assert.ErrorIs(t, err, ErrInvalidConsentStatus)
}

func TestNewConsent_ObtainedRequiresSource(t *testing.T) {
	_, err := NewConsent(ConsentStatusObtained, "   ", consentAt)
	assert.ErrorIs(t, err, ErrConsentSourceRequired)
}

func TestNewConsent_WithdrawnRequiresTimestamp(t *testing.T) {
	_, err := NewConsent(ConsentStatusWithdrawn, "unsubscribe", time.Time{})
	assert.ErrorIs(t, err, ErrConsentTimeRequired)
}

func TestNewOutboundOverride_RequiresReason(t *testing.T) {
	_, err := NewOutboundOverride("  ")
	assert.ErrorIs(t, err, ErrOverrideReasonRequired)

	ov, err := NewOutboundOverride("cold outreach, legitimate interest")
	require.NoError(t, err)
	assert.Equal(t, "cold outreach, legitimate interest", ov.Reason)
}

func prospectWithConsent(t *testing.T, status ConsentStatus) *Prospect {
	t.Helper()
	p, err := NewProspect(uuid.New(), "Acme", "Acme Inc", "CTO", "a@acme.io", "manual")
	require.NoError(t, err)
	p.Consent = Consent{Status: status, Source: "test", Timestamp: consentAt}
	return p
}

func TestAuthorizeOutbound_Obtained(t *testing.T) {
	p := prospectWithConsent(t, ConsentStatusObtained)
	assert.NoError(t, p.AuthorizeOutbound(nil))
}

func TestAuthorizeOutbound_WithdrawnIsHardBlock(t *testing.T) {
	p := prospectWithConsent(t, ConsentStatusWithdrawn)
	// Even a valid override cannot lift a withdrawal.
	ov, _ := NewOutboundOverride("please let me")
	assert.ErrorIs(t, p.AuthorizeOutbound(&ov), ErrConsentWithdrawn)
	assert.ErrorIs(t, p.AuthorizeOutbound(nil), ErrConsentWithdrawn)
}

func TestAuthorizeOutbound_NoneRequiresOverride(t *testing.T) {
	p := prospectWithConsent(t, ConsentStatusNone)
	assert.ErrorIs(t, p.AuthorizeOutbound(nil), ErrConsentRequired)

	ov, _ := NewOutboundOverride("cold outreach, legitimate interest")
	assert.NoError(t, p.AuthorizeOutbound(&ov))
}

func TestGrantConsent(t *testing.T) {
	p := prospectWithConsent(t, ConsentStatusNone)
	require.NoError(t, p.GrantConsent("inbound_reply", consentAt))
	assert.Equal(t, ConsentStatusObtained, p.Consent.Status)
	assert.Equal(t, "inbound_reply", p.Consent.Source)
	assert.NoError(t, p.AuthorizeOutbound(nil))
}

func TestWithdrawConsent(t *testing.T) {
	p := prospectWithConsent(t, ConsentStatusObtained)
	require.NoError(t, p.WithdrawConsent("unsubscribe", consentAt))
	assert.Equal(t, ConsentStatusWithdrawn, p.Consent.Status)
	assert.ErrorIs(t, p.AuthorizeOutbound(nil), ErrConsentWithdrawn)
}
