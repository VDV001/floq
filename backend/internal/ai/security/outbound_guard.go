package security

import "regexp"

// OutboundGuard is the agent-security-defaults layer-3 validator for the cold
// outbound Sender. It is distinct from ToolCallFirewall (which gates
// agent-initiated REPLY actions and keys off the inbound InputSeverity): a
// cold sequence send has no triggering inbound payload and its recipients are
// arbitrary external domains by design, so the relevant checks are structural:
//
//   - channel must be one the deployment allows;
//   - recipient must be well-formed for that channel (no sending to a
//     malformed/empty address);
//   - a batch larger than the mass-send threshold needs an explicit
//     out-of-band confirmation (deployment flag) before it may dispatch —
//     bounding blast radius for a destructive mass action.
//
// Context-free: operates on plain strings/ints, wired into the Sender via a
// port + adapter, never imported into the outbound domain directly.
type OutboundGuard struct {
	policy OutboundPolicy
}

// OutboundPolicy is the per-deployment configuration.
type OutboundPolicy struct {
	// AllowedChannels: channels permitted to send. Empty = none allowed
	// (fail-closed for an unconfigured deployment).
	AllowedChannels []string
	// MassSendThreshold: batch size above which a dispatch is considered a
	// mass send. 0 disables the check.
	MassSendThreshold int
	// MassSendConfirmed: the out-of-band confirmation that a mass send is
	// intended. Until set, an over-threshold batch is refused.
	MassSendConfirmed bool
}

// OutboundDecision is the guard verdict. Allowed=false is a hard skip.
type OutboundDecision struct {
	Allowed bool
	Reason  string
}

func NewOutboundGuard(policy OutboundPolicy) *OutboundGuard {
	return &OutboundGuard{policy: policy}
}

var reValidEmail = regexp.MustCompile(`^[\w.+-]+@[\w-]+\.[\w.-]+$`)

// CheckRecipient validates one planned send's channel and recipient.
func (g *OutboundGuard) CheckRecipient(channel, recipient string) OutboundDecision {
	return OutboundDecision{}
}

// CheckBatch validates the size of a dispatch batch against the mass-send
// policy. Called once per send tick before the per-message loop.
func (g *OutboundGuard) CheckBatch(size int) OutboundDecision {
	return OutboundDecision{}
}
