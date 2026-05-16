package prospects

import (
	"time"

	"github.com/daniil/floq/internal/prospects/domain"
	"github.com/google/uuid"
)

// --- Response DTOs ---

type ProspectResponse struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	Name             string     `json:"name"`
	Company          string     `json:"company"`
	Title            string     `json:"title"`
	Email            string     `json:"email"`
	Phone            string     `json:"phone"`
	WhatsApp         string     `json:"whatsapp"`
	TelegramUsername string     `json:"telegram_username"`
	Industry         string     `json:"industry"`
	CompanySize      string     `json:"company_size"`
	Context          string     `json:"context"`
	Source           string     `json:"source"`
	SourceID         *uuid.UUID `json:"source_id,omitempty"`
	SourceName       string     `json:"source_name,omitempty"`
	Status           string     `json:"status"`
	VerifyStatus     string     `json:"verify_status"`
	VerifyScore      int        `json:"verify_score"`
	VerifyDetails    string     `json:"verify_details"`
	VerifiedAt       *time.Time `json:"verified_at,omitempty"`
	ConvertedLeadID  *uuid.UUID `json:"converted_lead_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// --- Mapping functions ---

// ProspectToResponse maps the plain Prospect entity to a response; the list
// projection ProspectWithSourceToResponse adds source_name from the read
// model assembled by the repository.
func ProspectToResponse(p *domain.Prospect) ProspectResponse {
	return ProspectResponse{
		ID:               p.ID,
		UserID:           p.UserID,
		Name:             p.Name,
		Company:          p.Company,
		Title:            p.Title,
		Email:            p.Email,
		Phone:            p.Phone,
		WhatsApp:         p.WhatsApp,
		TelegramUsername: p.TelegramUsername,
		Industry:         p.Industry,
		CompanySize:      p.CompanySize,
		Context:          p.Context,
		Source:           p.Source,
		SourceID:         p.SourceID,
		Status:           string(p.Status),
		VerifyStatus:     string(p.VerifyStatus),
		VerifyScore:      p.VerifyScore,
		VerifyDetails:    p.VerifyDetails,
		VerifiedAt:       p.VerifiedAt,
		ConvertedLeadID:  p.ConvertedLeadID,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
	}
}

func ProspectWithSourceToResponse(p *domain.ProspectWithSource) ProspectResponse {
	resp := ProspectToResponse(&p.Prospect)
	resp.SourceName = p.SourceName
	return resp
}

func ProspectsToResponse(prospects []domain.ProspectWithSource) []ProspectResponse {
	if prospects == nil {
		return []ProspectResponse{}
	}
	resp := make([]ProspectResponse, len(prospects))
	for i := range prospects {
		resp[i] = ProspectWithSourceToResponse(&prospects[i])
	}
	return resp
}

// SkippedRowResponse is the wire representation of a row that the CSV
// importer dropped.
type SkippedRowResponse struct {
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

// ImportReportResponse is the wire representation of an ImportCSV result.
// Skipped is guaranteed to be a non-nil slice so the frontend can iterate
// without nil-checks; the use case layer carries it as the natural Go
// nil-or-slice and the mapper coerces here.
type ImportReportResponse struct {
	Imported int                  `json:"imported"`
	Skipped  []SkippedRowResponse `json:"skipped"`
}

// ImportReportToResponse maps the use case ImportReport into its wire form.
func ImportReportToResponse(r *ImportReport) ImportReportResponse {
	skipped := make([]SkippedRowResponse, len(r.Skipped))
	for i, s := range r.Skipped {
		skipped[i] = SkippedRowResponse{Line: s.Line, Reason: s.Reason}
	}
	return ImportReportResponse{Imported: r.Imported, Skipped: skipped}
}
