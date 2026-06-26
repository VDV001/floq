package enrichment

import (
	"time"

	"github.com/daniil/floq/internal/enrichment/domain"
)

// statusNone is the response status when there is no enrichment for the domain
// (no row yet, or the email had no company domain). Distinct from the entity
// statuses so the UI can render "no data" without treating it as an error.
const statusNone = "none"

// ProfileResponse is the public shape of the scraped company profile.
type ProfileResponse struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Emails      []string `json:"emails"`
	Phones      []string `json:"phones"`
	Socials     []string `json:"socials"`
	// Phase-2 (#186) LLM-classified fields. Empty strings when unclassified.
	Industry    string `json:"industry"`
	CompanySize string `json:"companySize"`
	// Phase-3 (#188) registry details. Zero value when not looked up.
	Legal LegalResponse `json:"legal"`
}

// LegalResponse is the public shape of a company's registry details.
type LegalResponse struct {
	INN      string `json:"inn"`
	OGRN     string `json:"ogrn"`
	FullName string `json:"fullName"`
	Address  string `json:"address"`
	OKVED    string `json:"okved"`
	Status   string `json:"status"`
}

// EnrichmentResponse is the read DTO returned by GET /api/enrichment.
type EnrichmentResponse struct {
	Domain     string          `json:"domain"`
	Status     string          `json:"status"`
	Profile    ProfileResponse `json:"profile"`
	EnrichedAt *time.Time      `json:"enrichedAt,omitempty"`
}

// noneResponse builds the "nothing to show" response for a domain.
func noneResponse(domainName string) EnrichmentResponse {
	return EnrichmentResponse{Domain: domainName, Status: statusNone}
}

// toResponse maps a domain entity to the read DTO.
func toResponse(e *domain.CompanyEnrichment) EnrichmentResponse {
	return EnrichmentResponse{
		Domain: e.Domain.String(),
		Status: e.Status.String(),
		Profile: ProfileResponse{
			Title:       e.Profile.Title,
			Description: e.Profile.Description,
			Emails:      e.Profile.Emails,
			Phones:      e.Profile.Phones,
			Socials:     e.Profile.Socials,
			Industry:    e.Profile.Industry,
			CompanySize: string(e.Profile.CompanySize),
			Legal: LegalResponse{
				INN:      e.Profile.Legal.INN,
				OGRN:     e.Profile.Legal.OGRN,
				FullName: e.Profile.Legal.FullName,
				Address:  e.Profile.Legal.Address,
				OKVED:    e.Profile.Legal.OKVED,
				Status:   e.Profile.Legal.Status,
			},
		},
		EnrichedAt: e.EnrichedAt,
	}
}
