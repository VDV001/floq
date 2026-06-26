package domain

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
