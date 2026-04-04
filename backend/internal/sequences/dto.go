package sequences

import (
	"time"

	"github.com/daniil/floq/internal/sequences/domain"
	"github.com/google/uuid"
)

// --- Response DTOs ---

type SequenceResponse struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Name      string    `json:"name"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type SequenceStepResponse struct {
	ID         uuid.UUID `json:"id"`
	SequenceID uuid.UUID `json:"sequence_id"`
	StepOrder  int       `json:"step_order"`
	DelayDays  int       `json:"delay_days"`
	PromptHint string    `json:"prompt_hint"`
	Channel    string    `json:"channel"`
	CreatedAt  time.Time `json:"created_at"`
}

type OutboundMessageResponse struct {
	ID          uuid.UUID  `json:"id"`
	ProspectID  uuid.UUID  `json:"prospect_id"`
	SequenceID  uuid.UUID  `json:"sequence_id"`
	StepOrder   int        `json:"step_order"`
	Channel     string     `json:"channel"`
	Body        string     `json:"body"`
	Status      string     `json:"status"`
	ScheduledAt time.Time  `json:"scheduled_at"`
	SentAt      *time.Time `json:"sent_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type StatsResponse struct {
	Draft    int `json:"draft"`
	Approved int `json:"approved"`
	Sent     int `json:"sent"`
	Opened   int `json:"opened"`
	Replied  int `json:"replied"`
	Bounced  int `json:"bounced"`
}

type SequenceDetailResponse struct {
	Sequence SequenceResponse       `json:"sequence"`
	Steps    []SequenceStepResponse `json:"steps"`
}

// --- Mapping functions ---

func SequenceToResponse(s *domain.Sequence) SequenceResponse {
	return SequenceResponse{
		ID:        s.ID,
		UserID:    s.UserID,
		Name:      s.Name,
		IsActive:  s.IsActive,
		CreatedAt: s.CreatedAt,
	}
}

func SequencesToResponse(seqs []domain.Sequence) []SequenceResponse {
	resp := make([]SequenceResponse, len(seqs))
	for i := range seqs {
		resp[i] = SequenceToResponse(&seqs[i])
	}
	return resp
}

func StepToResponse(s *domain.SequenceStep) SequenceStepResponse {
	return SequenceStepResponse{
		ID:         s.ID,
		SequenceID: s.SequenceID,
		StepOrder:  s.StepOrder,
		DelayDays:  s.DelayDays,
		PromptHint: s.PromptHint,
		Channel:    string(s.Channel),
		CreatedAt:  s.CreatedAt,
	}
}

func StepsToResponse(steps []domain.SequenceStep) []SequenceStepResponse {
	resp := make([]SequenceStepResponse, len(steps))
	for i := range steps {
		resp[i] = StepToResponse(&steps[i])
	}
	return resp
}

func OutboundMessageToResponse(m *domain.OutboundMessage) OutboundMessageResponse {
	return OutboundMessageResponse{
		ID:          m.ID,
		ProspectID:  m.ProspectID,
		SequenceID:  m.SequenceID,
		StepOrder:   m.StepOrder,
		Channel:     string(m.Channel),
		Body:        m.Body,
		Status:      string(m.Status),
		ScheduledAt: m.ScheduledAt,
		SentAt:      m.SentAt,
		CreatedAt:   m.CreatedAt,
	}
}

func OutboundMessagesToResponse(msgs []domain.OutboundMessage) []OutboundMessageResponse {
	resp := make([]OutboundMessageResponse, len(msgs))
	for i := range msgs {
		resp[i] = OutboundMessageToResponse(&msgs[i])
	}
	return resp
}

func StatsToResponse(s *domain.Stats) StatsResponse {
	return StatsResponse{
		Draft:    s.Draft,
		Approved: s.Approved,
		Sent:     s.Sent,
		Opened:   s.Opened,
		Replied:  s.Replied,
		Bounced:  s.Bounced,
	}
}
