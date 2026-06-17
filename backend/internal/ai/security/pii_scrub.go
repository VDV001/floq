package security

import (
	"fmt"
	"regexp"
	"strings"
)

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

var (
	rePIIEmail = regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.-]+`)
	// RU phone: +7 / 8 country prefix then 10 digits with optional separators.
	rePIIPhone = regexp.MustCompile(`(?:\+7|8)[\s\-]?\(?\d{3}\)?[\s\-]?\d{3}[\s\-]?\d{2}[\s\-]?\d{2}`)
	// ИНН: 12 (individual) or 10 (legal entity) standalone digits. Matched
	// after phones so a phone's digit run is already consumed.
	rePIIInn = regexp.MustCompile(`\b\d{12}\b|\b\d{10}\b`)
	// ФИО heuristic: three capitalized Cyrillic words (Фамилия Имя Отчество).
	// Conservative on purpose — two-word names are not matched to avoid
	// redacting ordinary capitalized phrases. No \b: in RE2 word boundaries
	// are ASCII-only, so Cyrillic gets none — the capitalization structure
	// is the anchor instead.
	rePIIName = regexp.MustCompile(`[А-ЯЁ][а-яё]+(?:\s+[А-ЯЁ][а-яё]+){2}`)
)

// piiRule pairs a placeholder type tag with its detector. Order matters:
// email and phone are consumed before ИНН so their digit runs can't be
// mis-tagged as ИНН.
type piiRule struct {
	tag string
	re  *regexp.Regexp
}

var piiRules = []piiRule{
	{"EMAIL", rePIIEmail},
	{"PHONE", rePIIPhone},
	{"INN", rePIIInn},
	{"NAME", rePIIName},
}

// Scrub replaces detected PII with placeholders ([EMAIL_1], [PHONE_1],
// [INN_1], [NAME_1]). Repeated occurrences of the same value collapse to the
// same placeholder.
func (s *PIIScrubber) Scrub(text string) ScrubResult {
	mapping := map[string]string{}
	seen := map[string]string{} // original → placeholder (dedup)
	counter := map[string]int{}

	for _, rule := range piiRules {
		text = rule.re.ReplaceAllStringFunc(text, func(match string) string {
			if ph, ok := seen[match]; ok {
				return ph
			}
			counter[rule.tag]++
			ph := fmt.Sprintf("[%s_%d]", rule.tag, counter[rule.tag])
			seen[match] = ph
			mapping[ph] = match
			return ph
		})
	}

	if len(mapping) == 0 {
		return ScrubResult{Scrubbed: text, Mapping: nil}
	}
	return ScrubResult{Scrubbed: text, Mapping: mapping}
}

// Restore re-hydrates placeholders in text back to their original values using
// the mapping produced by Scrub. The trailing "]" in every placeholder makes
// the replacements unambiguous ("[EMAIL_1]" is never a prefix of "[EMAIL_10]").
func (s *PIIScrubber) Restore(text string, mapping map[string]string) string {
	for placeholder, original := range mapping {
		text = strings.ReplaceAll(text, placeholder, original)
	}
	return text
}
