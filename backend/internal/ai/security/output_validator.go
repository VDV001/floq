package security

// OutputValidator is agent-security-defaults layer 2: it validates the LLM's
// qualification output against a strict structure before the result is trusted
// downstream. Three guarantees:
//
//   1. Range — Score is clamped to [0,100]; an out-of-range value is flagged
//      (a model returning 150 or -5 signals a parsing/jailbreak anomaly).
//   2. PII containment — any raw email/phone/ИНН/ФИО that leaked into the
//      reason text (despite input scrubbing) is redacted and flagged, so PII
//      never rides the qualification into the UI or audit log.
//   3. Confidence gate — a result below the confidence floor is forced to a
//      manual_review recommendation rather than an auto-engage action.
//
// It is context-free (operates on a local view struct), so it carries no
// dependency on the inbox bounded context.
type OutputValidator struct {
	minConfidence int
	scrubber      *PIIScrubber
}

// QualificationView is the security-local projection of a qualification result.
type QualificationView struct {
	Score             int
	ScoreReason       string
	RecommendedAction string
}

// OutputVerdict is the validated, possibly-corrected result plus an audit
// trail of what was changed.
type OutputVerdict struct {
	Score             int
	ScoreReason       string
	RecommendedAction string
	Flagged           bool
	Reasons           []string
}

// DefaultMinConfidence is the qualification score floor below which a result
// is downgraded to manual_review instead of being trusted to auto-engage.
const DefaultMinConfidence = 20

// NewOutputValidator builds a validator. Results scoring below minConfidence
// are downgraded to manual_review.
func NewOutputValidator(minConfidence int) *OutputValidator {
	return &OutputValidator{minConfidence: minConfidence, scrubber: NewPIIScrubber()}
}

// Validate applies the three guarantees and returns the corrected result.
func (v *OutputValidator) Validate(q QualificationView) OutputVerdict {
	out := OutputVerdict{
		Score:             q.Score,
		ScoreReason:       q.ScoreReason,
		RecommendedAction: q.RecommendedAction,
	}

	// 1. Range clamp.
	if out.Score > 100 {
		out.Score = 100
		out.flag("score above range, clamped to 100")
	}
	if out.Score < 0 {
		out.Score = 0
		out.flag("score below range, clamped to 0")
	}

	// 2. PII containment — redact anything that leaked into the reason.
	if scrub := v.scrubber.Scrub(out.ScoreReason); len(scrub.Mapping) > 0 {
		out.ScoreReason = scrub.Scrubbed
		out.flag("PII redacted from score reason")
	}

	// 3. Confidence gate — low score must not auto-engage.
	if out.Score < v.minConfidence {
		out.RecommendedAction = "manual_review"
		out.flag("score below confidence floor, downgraded to manual_review")
	}

	return out
}

func (o *OutputVerdict) flag(reason string) {
	o.Flagged = true
	o.Reasons = append(o.Reasons, reason)
}
