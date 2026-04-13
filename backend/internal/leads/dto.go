package leads

import (
	"time"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
)

// --- Response DTOs ---

type LeadResponse struct {
	ID             uuid.UUID `json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	Channel        string    `json:"channel"`
	ContactName    string    `json:"contact_name"`
	Company        string    `json:"company"`
	FirstMessage   string    `json:"first_message"`
	Status         string    `json:"status"`
	TelegramChatID *int64    `json:"telegram_chat_id,omitempty"`
	EmailAddress   *string    `json:"email_address,omitempty"`
	SourceID       *uuid.UUID `json:"source_id,omitempty"`
	SourceName     string     `json:"source_name,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type MessageResponse struct {
	ID        uuid.UUID `json:"id"`
	LeadID    uuid.UUID `json:"lead_id"`
	Direction string    `json:"direction"`
	Body      string    `json:"body"`
	SentAt    time.Time `json:"sent_at"`
}

type QualificationResponse struct {
	ID                uuid.UUID `json:"id"`
	LeadID            uuid.UUID `json:"lead_id"`
	IdentifiedNeed    string    `json:"identified_need"`
	EstimatedBudget   string    `json:"estimated_budget"`
	Deadline          string    `json:"deadline"`
	Score             int       `json:"score"`
	ScoreReason       string    `json:"score_reason"`
	RecommendedAction string    `json:"recommended_action"`
	ProviderUsed      string    `json:"provider_used"`
	GeneratedAt       time.Time `json:"generated_at"`
}

type DraftResponse struct {
	ID        uuid.UUID `json:"id"`
	LeadID    uuid.UUID `json:"lead_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// --- Mapping functions ---

func LeadToResponse(l *domain.Lead) LeadResponse {
	return LeadResponse{
		ID:             l.ID,
		UserID:         l.UserID,
		Channel:        string(l.Channel),
		ContactName:    l.ContactName,
		Company:        l.Company,
		FirstMessage:   l.FirstMessage,
		Status:         string(l.Status),
		TelegramChatID: l.TelegramChatID,
		EmailAddress:   l.EmailAddress,
		SourceID:       l.SourceID,
		CreatedAt:      l.CreatedAt,
		UpdatedAt:      l.UpdatedAt,
	}
}

func LeadsToResponse(leads []domain.Lead) []LeadResponse {
	resp := make([]LeadResponse, len(leads))
	for i := range leads {
		resp[i] = LeadToResponse(&leads[i])
	}
	return resp
}

func MessageToResponse(m *domain.Message) MessageResponse {
	return MessageResponse{
		ID:        m.ID,
		LeadID:    m.LeadID,
		Direction: string(m.Direction),
		Body:      m.Body,
		SentAt:    m.SentAt,
	}
}

func MessagesToResponse(msgs []domain.Message) []MessageResponse {
	resp := make([]MessageResponse, len(msgs))
	for i := range msgs {
		resp[i] = MessageToResponse(&msgs[i])
	}
	return resp
}

func QualificationToResponse(q *domain.Qualification) QualificationResponse {
	return QualificationResponse{
		ID:                q.ID,
		LeadID:            q.LeadID,
		IdentifiedNeed:    q.IdentifiedNeed,
		EstimatedBudget:   q.EstimatedBudget,
		Deadline:          q.Deadline,
		Score:             q.Score,
		ScoreReason:       q.ScoreReason,
		RecommendedAction: q.RecommendedAction,
		ProviderUsed:      q.ProviderUsed,
		GeneratedAt:       q.GeneratedAt,
	}
}

func DraftToResponse(d *domain.Draft) DraftResponse {
	return DraftResponse{
		ID:        d.ID,
		LeadID:    d.LeadID,
		Body:      d.Body,
		CreatedAt: d.CreatedAt,
	}
}
