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
	TelegramUsername string     `json:"telegram_username"`
	Industry         string     `json:"industry"`
	CompanySize      string     `json:"company_size"`
	Context          string     `json:"context"`
	Source           string     `json:"source"`
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

func ProspectToResponse(p *domain.Prospect) ProspectResponse {
	return ProspectResponse{
		ID:               p.ID,
		UserID:           p.UserID,
		Name:             p.Name,
		Company:          p.Company,
		Title:            p.Title,
		Email:            p.Email,
		Phone:            p.Phone,
		TelegramUsername: p.TelegramUsername,
		Industry:         p.Industry,
		CompanySize:      p.CompanySize,
		Context:          p.Context,
		Source:           p.Source,
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

func ProspectsToResponse(prospects []domain.Prospect) []ProspectResponse {
	resp := make([]ProspectResponse, len(prospects))
	for i := range prospects {
		resp[i] = ProspectToResponse(&prospects[i])
	}
	return resp
}
