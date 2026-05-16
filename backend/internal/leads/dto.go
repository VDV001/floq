package leads

import (
	"time"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
)

// --- Response DTOs ---

type LeadResponse struct {
	ID             uuid.UUID                `json:"id"`
	UserID         uuid.UUID                `json:"user_id"`
	Channel        string                   `json:"channel"`
	ContactName    string                   `json:"contact_name"`
	Company        string                   `json:"company"`
	FirstMessage   string                   `json:"first_message"`
	Status         string                   `json:"status"`
	TelegramChatID *int64                   `json:"telegram_chat_id,omitempty"`
	EmailAddress   *string                  `json:"email_address,omitempty"`
	SourceID       *uuid.UUID               `json:"source_id,omitempty"`
	SourceName     string                   `json:"source_name,omitempty"`
	CreatedAt      time.Time                `json:"created_at"`
	UpdatedAt      time.Time                `json:"updated_at"`
	Identity       *IdentitySummaryResponse `json:"identity,omitempty"`
}

// IdentitySummaryResponse projects the unified-identity context onto
// the lead detail page. LinkedLeadIDs always includes the requesting
// lead — clients dedupe when rendering the IdentityBadge sibling list.
// All identifier fields are pre-canonicalized server-side; the
// frontend renders them as-is.
type IdentitySummaryResponse struct {
	ID               uuid.UUID   `json:"id"`
	Email            string      `json:"email,omitempty"`
	Phone            string      `json:"phone,omitempty"`
	TelegramUsername string      `json:"telegram_username,omitempty"`
	LinkedLeadIDs    []uuid.UUID `json:"linked_lead_ids"`
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

// LeadToResponse maps the plain Lead entity to a response (single-lead endpoints
// omit source_name since it's not loaded — clients get source_id only).
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

// LeadViewToResponse maps the full identity-aware view used by the
// detail page. Identity is omitted from the JSON (omitempty) when the
// lead has no linked Identity yet — the frontend treats absence as
// "single-channel lead" and hides the IdentityBadge.
func LeadViewToResponse(v *LeadView) LeadResponse {
	resp := LeadToResponse(v.Lead)
	if v.Identity != nil {
		linked := v.LinkedLeadIDs
		if linked == nil {
			linked = []uuid.UUID{}
		}
		resp.Identity = &IdentitySummaryResponse{
			ID:               v.Identity.ID,
			Email:            v.Identity.Email,
			Phone:            v.Identity.Phone,
			TelegramUsername: v.Identity.TelegramUsername,
			LinkedLeadIDs:    linked,
		}
	}
	return resp
}

// LeadWithSourceToResponse projects the list read-model including source_name.
func LeadWithSourceToResponse(l *domain.LeadWithSource) LeadResponse {
	resp := LeadToResponse(&l.Lead)
	resp.SourceName = l.SourceName
	return resp
}

func LeadsToResponse(leads []domain.LeadWithSource) []LeadResponse {
	resp := make([]LeadResponse, len(leads))
	for i := range leads {
		resp[i] = LeadWithSourceToResponse(&leads[i])
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
