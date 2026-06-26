package domain

import "strings"

// CompanySize is the headcount bucket of a company, part of the ubiquitous
// language of the enrichment context. It is a closed set (typed enum, not a
// magic string) so the LLM extractor must classify into one of the known
// buckets; anything else is rejected and treated as Unknown. Serialized as a
// plain JSON string inside CompanyProfile, so the zero value MUST be the
// Unknown bucket — an un-enriched or legacy profile then deserializes to
// "no data", never to a wrong classification.
type CompanySize string

const (
	CompanySizeUnknown    CompanySize = ""           // not classified / no data
	CompanySizeSolo       CompanySize = "solo"       // 1
	CompanySizeSmall      CompanySize = "small"      // 2–10
	CompanySizeMedium     CompanySize = "medium"     // 11–50
	CompanySizeLarge      CompanySize = "large"      // 51–250
	CompanySizeEnterprise CompanySize = "enterprise" // 250+
)

// ParseCompanySize normalizes an untrusted label (e.g. from the LLM) into a
// known bucket: lowercased, trimmed, and validated. Anything not a known
// bucket — including the empty string — collapses to Unknown. This keeps the
// "what counts as a valid size" invariant inside the domain rather than in the
// extractor.
func ParseCompanySize(s string) CompanySize {
	size := CompanySize(strings.ToLower(strings.TrimSpace(s)))
	if !size.IsValid() {
		return CompanySizeUnknown
	}
	return size
}

// IsValid reports whether s is a known bucket. Unknown (the zero value) is
// valid: it is the legitimate "no data" state, not a malformed value.
func (s CompanySize) IsValid() bool {
	switch s {
	case CompanySizeUnknown, CompanySizeSolo, CompanySizeSmall,
		CompanySizeMedium, CompanySizeLarge, CompanySizeEnterprise:
		return true
	default:
		return false
	}
}
