package domain

import "strings"

// MaxIndustryLen caps the stored industry label. Industry is an open
// taxonomy (we keep a normalized free-text VO rather than a brittle closed
// enum), but the value still has an invariant: a short, normalized label —
// never a paragraph the LLM hallucinated or an injected dump.
const MaxIndustryLen = 64

// NormalizeIndustry enforces the Industry value-object invariant: lowercased,
// trimmed, inner whitespace collapsed to single spaces, and capped to
// MaxIndustryLen runes. Empty/whitespace-only input normalizes to "" (no
// data). This is the single place industry strings are sanitized before they
// enter a CompanyProfile, so an LLM (fed untrusted scraped HTML) cannot stuff
// an unbounded or oddly-spaced value into the profile.
func NormalizeIndustry(s string) string {
	// strings.Fields splits on any run of whitespace, so Join with a single
	// space both trims the ends and collapses interior runs.
	s = strings.ToLower(strings.Join(strings.Fields(s), " "))
	if r := []rune(s); len(r) > MaxIndustryLen {
		s = string(r[:MaxIndustryLen])
	}
	return s
}
