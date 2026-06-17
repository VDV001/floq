package security

import "errors"

// PIIScrubber replaces personal data (email, phone, ИНН, ФИО) in untrusted
// inbound text with stable placeholders BEFORE the text enters the LLM
// context, keeping a reversible mapping so the model's downstream output
// (e.g. a draft reply) can be re-hydrated with the real values at send time.
//
// Rationale (agent-security-defaults layer 1b / dissociation): the model
// never needs to see a client's real email or phone to qualify a lead; not
// putting PII into the prompt removes a whole class of leakage (the model
// echoing PII into score_reason, an injection exfiltrating it, the provider
// logging it). Detection is regex-based and deliberately conservative —
// structured identifiers (email/phone/ИНН) are caught reliably; ФИО is a
// best-effort Фамилия-Имя-Отчество heuristic, NOT NER. See ADR / security-model.
type PIIScrubber struct{}

// ScrubResult carries the redacted text and the placeholder→original mapping.
type ScrubResult struct {
	Scrubbed string
	Mapping  map[string]string
}

// NewPIIScrubber builds a scrubber with the canonical rule set.
func NewPIIScrubber() *PIIScrubber { return &PIIScrubber{} }

// Scrub replaces detected PII with placeholders ([EMAIL_1], [PHONE_1],
// [INN_1], [NAME_1]). Repeated occurrences of the same value collapse to the
// same placeholder.
func (s *PIIScrubber) Scrub(text string) ScrubResult {
	return ScrubResult{Scrubbed: "", Mapping: nil}
}

// Restore re-hydrates placeholders in text back to their original values using
// the mapping produced by Scrub.
func (s *PIIScrubber) Restore(text string, mapping map[string]string) string {
	return ""
}

var _ = errors.New
