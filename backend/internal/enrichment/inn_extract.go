package enrichment

import (
	"regexp"

	"github.com/daniil/floq/internal/enrichment/domain"
)

// innLabelRe finds a 10–12 digit run that immediately follows an "ИНН" label
// (case-insensitive), allowing a few non-digit separators (":", space, nbsp).
// Requiring the label is the precision guard: a bare 10-digit number on a page
// (an order id, a phone) must NOT be treated as an INN. The checksum in
// domain.NewINN is the second guard.
var innLabelRe = regexp.MustCompile(`(?i)инн\D{0,5}(\d{10,12})`)

// ExtractINN returns the first checksum-valid INN that appears right after an
// "ИНН" label on the page, or "" if none. The result is already validated, so
// it is safe to use as a precise registry-lookup key.
func ExtractINN(page string) string {
	for _, m := range innLabelRe.FindAllStringSubmatch(page, -1) {
		if inn, err := domain.NewINN(m[1]); err == nil {
			return inn.String()
		}
	}
	return ""
}
