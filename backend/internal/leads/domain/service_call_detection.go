package domain

import "strings"

// --- CallIntentDetector domain service ---
//
// A domain *service* rather than an entity method: the rule ("does this
// message indicate the speaker agreed to a call or meeting?") doesn't
// naturally live on any single entity (it's about Message text, but tuning
// the markers has nothing to do with Message's invariants). The separate
// file name signals the category — see Evans' "Domain Services" chapter.

// callAgreementMarkers is the Russian-language vocabulary that indicates a
// prospect has agreed to a call or meeting. Stored as package state so the
// matcher is allocation-free and the list is amenable to tuning as a whole.
var callAgreementMarkers = []string{
	"давайте созвон", "давай созвон", "готов созвон", "согласен на созвон",
	"можно созвон", "давайте звонок", "давай звонок", "готов к звонку",
	"давайте встреч", "давай встреч", "согласен на встреч", "готов встретить",
	"можем созвон", "можем встретить", "давайте обсудим", "готов обсудить",
	"да, давайте", "да давайте", "конечно, давайте", "с удовольствием",
	"когда удобно", "выберу время", "забронир", "запишусь",
	"да, можно", "да можно", "ок, давай", "ок давай",
}

// DetectCallAgreement is a stateless domain service that reports whether a
// message body expresses agreement to a call or meeting. Returns true on the
// first marker match; case-insensitive. Pure function — no side effects, no
// external lookups.
func DetectCallAgreement(text string) bool {
	lower := strings.ToLower(text)
	for _, m := range callAgreementMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}
