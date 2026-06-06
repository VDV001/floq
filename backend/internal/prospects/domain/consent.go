package domain

import (
	"errors"
	"strings"
	"time"
)

// ConsentStatus is the prospect's outbound-contact consent state. It is a
// first-class compliance concept (152-ФЗ / 38-ФЗ / GDPR-style): every send
// decision is justified against it and the justification is auditable.
//
//   - none      — no recorded basis. Cold outreach: allowed ONLY with an
//                 explicit, logged OutboundOverride (the lawful-basis record).
//   - obtained  — the prospect consented (opted in, replied to us, or a
//                 declared import). Send freely.
//   - withdrawn — the prospect opted out / unsubscribed. NEVER send. This is
//                 the absolute red line; no override can lift it.
type ConsentStatus string

const (
	ConsentStatusNone      ConsentStatus = "none"
	ConsentStatusObtained  ConsentStatus = "obtained"
	ConsentStatusWithdrawn ConsentStatus = "withdrawn"
)

// IsValid reports whether s is a known consent status.
func (s ConsentStatus) IsValid() bool {
	switch s {
	case ConsentStatusNone, ConsentStatusObtained, ConsentStatusWithdrawn:
		return true
	default:
		return false
	}
}

func (s ConsentStatus) String() string { return string(s) }

// Domain errors. ErrConsentWithdrawn and ErrConsentRequired are the two
// send-time rejections; the rest guard VO construction.
var (
	ErrInvalidConsentStatus   = errors.New("consent: invalid status")
	ErrConsentSourceRequired  = errors.New("consent: source is required for obtained/withdrawn")
	ErrConsentTimeRequired    = errors.New("consent: timestamp is required for obtained/withdrawn")
	ErrOverrideReasonRequired = errors.New("consent: override reason is required")

	// ErrConsentWithdrawn is returned when outbound is attempted to a prospect
	// who opted out. Hard block — an override cannot lift it.
	ErrConsentWithdrawn = errors.New("consent: prospect has withdrawn consent")
	// ErrConsentRequired is returned when outbound is attempted to a prospect
	// with no recorded consent and no lawful-basis override.
	ErrConsentRequired = errors.New("consent: obtained consent or a logged override is required")
)

// Consent is an immutable value object recording a prospect's consent state,
// where it came from, and when it was recorded.
type Consent struct {
	Status    ConsentStatus
	Source    string // e.g. "legacy", "inbound_reply", "import", "manual", "unsubscribe"
	Timestamp time.Time
}

// NewConsent builds a validated Consent VO. For obtained/withdrawn a non-empty
// source and a non-zero timestamp are mandatory (auditability); for none they
// are optional and normalized away.
func NewConsent(status ConsentStatus, source string, at time.Time) (Consent, error) {
	if !status.IsValid() {
		return Consent{}, ErrInvalidConsentStatus
	}
	source = strings.TrimSpace(source)
	if status == ConsentStatusNone {
		// No basis recorded — source/timestamp carry no meaning, normalize away.
		return Consent{Status: ConsentStatusNone}, nil
	}
	if source == "" {
		return Consent{}, ErrConsentSourceRequired
	}
	if at.IsZero() {
		return Consent{}, ErrConsentTimeRequired
	}
	return Consent{Status: status, Source: source, Timestamp: at.UTC()}, nil
}

// OutboundOverride is the logged lawful-basis record that authorizes a cold
// send to a none-consent prospect. It is never a silent bypass — the reason is
// mandatory and meant to be persisted to the audit trail.
type OutboundOverride struct {
	Reason string
}

// NewOutboundOverride builds a validated override; the reason must be non-empty.
func NewOutboundOverride(reason string) (OutboundOverride, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return OutboundOverride{}, ErrOverrideReasonRequired
	}
	return OutboundOverride{Reason: reason}, nil
}

// AuthorizeOutbound is the single domain rule gating outbound sends:
//   - withdrawn → ErrConsentWithdrawn always (override ignored);
//   - obtained  → authorized;
//   - none       → authorized only if a valid override is supplied, else
//                  ErrConsentRequired.
func (p *Prospect) AuthorizeOutbound(override *OutboundOverride) error {
	switch p.Consent.Status {
	case ConsentStatusWithdrawn:
		return ErrConsentWithdrawn
	case ConsentStatusObtained:
		return nil
	default: // none or unset — cold contact
		if override != nil && strings.TrimSpace(override.Reason) != "" {
			return nil
		}
		return ErrConsentRequired
	}
}

// GrantConsent records obtained consent from the given source at time at.
// A withdrawal is sticky: re-granting is a deliberate fresh opt-in and is
// allowed, but it is recorded with its own source/timestamp for the trail.
func (p *Prospect) GrantConsent(source string, at time.Time) error {
	c, err := NewConsent(ConsentStatusObtained, source, at)
	if err != nil {
		return err
	}
	p.Consent = c
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// WithdrawConsent records that the prospect opted out (e.g. via unsubscribe).
func (p *Prospect) WithdrawConsent(source string, at time.Time) error {
	c, err := NewConsent(ConsentStatusWithdrawn, source, at)
	if err != nil {
		return err
	}
	p.Consent = c
	p.UpdatedAt = time.Now().UTC()
	return nil
}
