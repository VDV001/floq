// Package security implements pre-LLM defensive layers for Floq's
// AI-powered inbox quality pipeline (lead qualification, draft
// generation). Following KB standard agent-security-defaults v1.0 and
// the Doubletapp Meridian benchmark (50% prompt-injection pass rate
// with system-prompt-only defence), the architectural fence is moved
// *before* the LLM via the InputFirewall.
//
// Key principle: do not rely on the system prompt to enforce safety.
// Parse the inbound payload in code, classify it, and decide whether
// to (a) pass through unchanged, (b) pass through with a warn-flag for
// downstream humans, or (c) block before the model sees it.
package security

import (
	"regexp"
	"strings"
)

// Severity classifies a firewall verdict. Info = pass-through, log
// only. Warn = pass-through, but downstream tools must treat the
// content as suspicious (e.g. don't auto-fire write-actions on it).
// Block = refuse to forward to the LLM.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarn
	SeverityBlock
)

// String returns the canonical lowercase name for telemetry.
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarn:
		return "warn"
	case SeverityBlock:
		return "block"
	default:
		return "unknown"
	}
}

// ScanResult is the firewall verdict on a single inbound payload. The
// Sanitized field carries a redacted version where blocked sections
// are replaced with "[BLOCKED:reason]" markers — useful for human
// review of the audit trail without re-exposing the attack payload.
type ScanResult struct {
	Allowed         bool
	Severity        Severity
	MatchedPatterns []string
	Reason          string
	Sanitized       string
}

// pattern is a single detection rule. Match returns the matched
// substring (for redaction) or empty string if no match.
type pattern struct {
	name     string
	severity Severity
	reason   string
	match    func(text string) string
}

// InputFirewall holds the rule set. Construct via NewInputFirewall —
// the rule list is package-private to ensure consistent ordering and
// test parity.
type InputFirewall struct {
	patterns []pattern
}

// regexpMatcher returns a closure that returns the matched substring
// (or "") for a given text. Wraps a precompiled regexp for speed —
// scan runs on every inbound message.
func regexpMatcher(re *regexp.Regexp) func(string) string {
	return func(text string) string {
		if loc := re.FindStringIndex(text); loc != nil {
			return text[loc[0]:loc[1]]
		}
		return ""
	}
}

// Pre-compiled rule set. Order matters only for which pattern name
// appears first in MatchedPatterns — verdict severity is monotonic
// (any block beats any warn beats any info).
var (
	// Block-severity: explicit jailbreak / role-override attempts.
	// Allows "ignore previous instructions", "disregard your prior rules",
	// "forget all the above messages". Optional possessive/quantifier
	// between verb and "previous/prior/above/earlier".
	reIgnorePrevious = regexp.MustCompile(`(?i)\b(ignore|disregard|forget)\s+(your|the|all|all\s+the|all\s+your)?\s*(previous|prior|above|earlier)\s+(instructions?|prompts?|rules?|messages?)`)
	reRussianIgnore  = regexp.MustCompile(`(?i)(забудь|игнорируй|забудьте|игнорируйте)\s+(все\s+)?(предыдущ|прежн|вышеупомян)`)
	reRussianNewInst = regexp.MustCompile(`(?i)(нов(ая|ые))\s+(инструкци|правил)`)
	// Multi-form: "Print|reveal|show ... (system|initial|original|your)
	// [adj] prompt/instructions" — allows up to 3 adjectives between
	// the determiner and the target noun ("your initial system prompt",
	// "the original instructions"). Also catches "repeat the words above"
	// and "...verbatim" markers commonly used to coax the model into
	// dumping its context window.
	rePromptExtract = regexp.MustCompile(`(?i)\b(print|repeat|reveal|show|tell|output|display)\b[^.!?\n]{0,80}\b(system|initial|original|your|the)\s+(\w+\s+){0,3}(prompt|instructions|rules)`)
	rePromptVerbatim = regexp.MustCompile(`(?i)\b(prompt|instructions?|context)\s+verbatim\b|\bverbatim,?\s+(including|with|all)\s+(all\s+)?(your\s+)?(instructions?|rules?|prompt|context)`)
	rePromptRepeatAbove = regexp.MustCompile(`(?i)\brepeat\s+the\s+words\s+above\b`)
	rePromptRussian = regexp.MustCompile(`(?i)(расскажи|покажи|раскрой|выведи)\s+(свой|твой)\s+(промпт|систем\w*|инструкци\w+)`)
	reRoleOverride   = regexp.MustCompile(`(?i)(\[SYSTEM\]|<\|im_start\|>\s*system|###\s*system\s*###|from\s+now\s+on,?\s+you\s+are|you\s+are\s+now\s+(a\s+different|in\s+admin)|твоё?\s+имя\s+теперь|роль:\s*(система|system))`)
	reEncodedJB      = regexp.MustCompile(`(?i)(decode|расшифруй).{0,20}(base64|основ\w*64).{0,20}(follow|выполни|и\s+follow)`)

	// Warn-severity: data-exfiltration shaped patterns. Pass through
	// (a real lead may legitimately ask for a callback URL or external
	// email handoff) but flag — downstream tools must not auto-fire
	// send_email / forward operations on warn-tagged inputs without
	// human confirmation.
	reExternalURL = regexp.MustCompile(`(?i)\b(send|post|forward|deliver|отправь|перешл[иё]|пошл[иё])\b[^.!?\n]{0,40}\b(http://|https://)[^\s]+`)
	reForwardData = regexp.MustCompile(`(?i)\b(forward|send|перешл[иё])\s+(all\s+|all\s+the\s+|все\s+|весь\s+)(customer|client|email|emails|user|данн|переписк|почт)`)
	reRedirectMail = regexp.MustCompile(`(?i)\b(send|forward|deliver|отправь|перешл[иё])\b[^.!?\n]{0,60}\b[\w._%+-]+@[\w.-]+\.[A-Za-z]{2,}`)
)

// NewInputFirewall constructs a firewall with the canonical Floq rule
// set. The constructor is parameterless on purpose — adding rules
// elsewhere makes the security posture diffuse and untestable.
func NewInputFirewall() *InputFirewall {
	return &InputFirewall{
		patterns: []pattern{
			{name: "jailbreak_ignore_previous", severity: SeverityBlock, reason: "instruction-override attempt", match: regexpMatcher(reIgnorePrevious)},
			{name: "jailbreak_russian_ignore", severity: SeverityBlock, reason: "instruction-override attempt (ru)", match: regexpMatcher(reRussianIgnore)},
			{name: "jailbreak_russian_new_instruction", severity: SeverityBlock, reason: "new-instruction attempt (ru)", match: regexpMatcher(reRussianNewInst)},
			{name: "jailbreak_role_override", severity: SeverityBlock, reason: "role/persona override", match: regexpMatcher(reRoleOverride)},
			{name: "jailbreak_prompt_extraction", severity: SeverityBlock, reason: "system-prompt extraction request", match: regexpMatcher(rePromptExtract)},
			{name: "jailbreak_prompt_verbatim", severity: SeverityBlock, reason: "verbatim-dump request", match: regexpMatcher(rePromptVerbatim)},
			{name: "jailbreak_prompt_repeat_above", severity: SeverityBlock, reason: "repeat-words-above attack", match: regexpMatcher(rePromptRepeatAbove)},
			{name: "jailbreak_prompt_russian", severity: SeverityBlock, reason: "system-prompt extraction (ru)", match: regexpMatcher(rePromptRussian)},
			{name: "jailbreak_encoded_payload", severity: SeverityBlock, reason: "base64-wrapped instruction", match: regexpMatcher(reEncodedJB)},
			{name: "exfiltration_external_url", severity: SeverityWarn, reason: "asks to send/forward to external URL", match: regexpMatcher(reExternalURL)},
			{name: "exfiltration_forward_data", severity: SeverityWarn, reason: "asks to forward customer/email data", match: regexpMatcher(reForwardData)},
			{name: "exfiltration_redirect_email", severity: SeverityWarn, reason: "asks to send to external email", match: regexpMatcher(reRedirectMail)},
		},
	}
}

// Scan runs every pattern against text. The result's Severity is the
// max across matches (Block dominates Warn dominates Info). Sanitized
// is the text with any matched substrings replaced by
// "[BLOCKED:reason]" markers — the redaction is deliberately verbose
// so audit-log readers see what was caught without having to re-decode
// the payload.
func (f *InputFirewall) Scan(text string) ScanResult {
	r := ScanResult{
		Allowed:   true,
		Severity:  SeverityInfo,
		Sanitized: text,
	}
	for _, p := range f.patterns {
		matched := p.match(text)
		if matched == "" {
			continue
		}
		r.MatchedPatterns = append(r.MatchedPatterns, p.name)
		if p.severity > r.Severity {
			r.Severity = p.severity
		}
		if p.severity == SeverityBlock {
			r.Allowed = false
			r.Sanitized = strings.Replace(r.Sanitized, matched, "[BLOCKED:"+p.reason+"]", 1)
			if r.Reason == "" {
				r.Reason = p.reason
			}
		}
	}
	if !r.Allowed && r.Reason == "" {
		r.Reason = "input firewall blocked"
	}
	return r
}
