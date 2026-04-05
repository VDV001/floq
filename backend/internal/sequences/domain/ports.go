package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository defines persistence operations for the sequences domain.
type Repository interface {
	// Sequences
	ListSequences(ctx context.Context, userID uuid.UUID) ([]Sequence, error)
	GetSequence(ctx context.Context, id uuid.UUID) (*Sequence, error)
	CreateSequence(ctx context.Context, s *Sequence) error
	UpdateSequence(ctx context.Context, s *Sequence) error
	DeleteSequence(ctx context.Context, id uuid.UUID) error
	ToggleActive(ctx context.Context, id uuid.UUID, active bool) error

	// Steps
	ListSteps(ctx context.Context, sequenceID uuid.UUID) ([]SequenceStep, error)
	CreateStep(ctx context.Context, step *SequenceStep) error

	// Outbound messages
	CreateOutboundMessage(ctx context.Context, msg *OutboundMessage) error
	ListOutboundQueue(ctx context.Context, userID uuid.UUID) ([]OutboundMessage, error)
	ListSentMessages(ctx context.Context, userID uuid.UUID) ([]OutboundMessage, error)
	UpdateOutboundStatus(ctx context.Context, id uuid.UUID, status OutboundStatus) error
	UpdateOutboundBody(ctx context.Context, id uuid.UUID, body string) error
	GetPendingSends(ctx context.Context) ([]OutboundMessage, error)
	MarkSent(ctx context.Context, id uuid.UUID) error
	MarkBounced(ctx context.Context, id uuid.UUID) error

	// Stats
	GetStats(ctx context.Context, userID uuid.UUID) (*Stats, error)
}

// AIMessageGenerator generates outbound messages using AI.
type AIMessageGenerator interface {
	GenerateColdMessage(ctx context.Context, name, title, company, prospectContext, stepHint, previousMessage string) (string, error)
	GenerateTelegramMessage(ctx context.Context, name, title, company, prospectContext, stepHint, previousMessage, source string) (string, error)
	GenerateCallBrief(ctx context.Context, name, title, company, prospectContext, stepHint, previousMessage string) (string, error)
}

// ProspectView is a read model for cross-context use of prospect data.
type ProspectView struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	Name             string
	Company          string
	Title            string
	Email            string
	Phone            string
	TelegramUsername string
	Context          string
	Source           string
	Status           string
	VerifyStatus     string
	VerifiedAt       *time.Time
}

// ProspectReader provides read access to prospect data from the sequences context.
type ProspectReader interface {
	GetProspect(ctx context.Context, id uuid.UUID) (*ProspectView, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}

// LeadCreator creates leads from converted prospects.
type LeadCreator interface {
	CreateLeadFromProspect(ctx context.Context, prospect *ProspectView, userID uuid.UUID) (uuid.UUID, error)
}

// TxManager provides transactional execution.
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
