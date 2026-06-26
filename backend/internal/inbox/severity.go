package inbox

// Severity is the InputFirewall verdict for an inbound message, carried
// alongside the PendingReply it triggered so the reply-dispatch gate can
// refuse to deliver a customer-visible message that was provoked by a
// blocked (jailbreak / extraction) payload. It mirrors the security
// package's severity ladder but is modelled independently here: the
// inbox context must not import internal/ai/security (the mapping lives
// in the composition-root adapter).
//
//   - Info  — pass-through, audit only.
//   - Warn  — suspicious; downstream destructive actions require an
//     explicit human confirm (the HITL approval queue satisfies this).
//   - Block — refuse to fan out into a customer-visible reply.
type Severity string

const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityBlock Severity = "block"
)

// IsValid reports whether the severity is one of the known values.
func (s Severity) IsValid() bool {
	switch s {
	case SeverityInfo, SeverityWarn, SeverityBlock:
		return true
	default:
		return false
	}
}

// String returns the underlying string for logging / persistence.
func (s Severity) String() string { return string(s) }
