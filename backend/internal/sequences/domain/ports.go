package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SequenceRepo manages sequence CRUD.
type SequenceRepo interface {
	ListSequences(ctx context.Context, userID uuid.UUID) ([]Sequence, error)
	GetSequence(ctx context.Context, id uuid.UUID) (*Sequence, error)
	CreateSequence(ctx context.Context, s *Sequence) error
	UpdateSequence(ctx context.Context, s *Sequence) error
	DeleteSequence(ctx context.Context, id uuid.UUID) error
	ToggleActive(ctx context.Context, id uuid.UUID, active bool) error
}

// StepRepo manages sequence step CRUD.
type StepRepo interface {
	ListSteps(ctx context.Context, sequenceID uuid.UUID) ([]SequenceStep, error)
	CreateStep(ctx context.Context, step *SequenceStep) error
	DeleteStep(ctx context.Context, stepID uuid.UUID) error
}

// OutboundMessageRepo manages outbound message operations.
type OutboundMessageRepo interface {
	CreateOutboundMessage(ctx context.Context, msg *OutboundMessage) error
	ListOutboundQueue(ctx context.Context, userID uuid.UUID) ([]OutboundMessage, error)
	ListSentMessages(ctx context.Context, userID uuid.UUID) ([]OutboundMessage, error)
	UpdateOutboundStatus(ctx context.Context, id uuid.UUID, status OutboundStatus) error
	UpdateOutboundBody(ctx context.Context, id uuid.UUID, body string) error
	GetPendingSends(ctx context.Context) ([]OutboundMessage, error)
	MarkSent(ctx context.Context, id uuid.UUID) error
	MarkBounced(ctx context.Context, id uuid.UUID) error
	MarkOpened(ctx context.Context, id uuid.UUID) error
	GetOutboundMessage(ctx context.Context, id uuid.UUID) (*OutboundMessage, error)
	GetStats(ctx context.Context, userID uuid.UUID) (*Stats, error)
	GetConversationHistory(ctx context.Context, prospectID uuid.UUID) ([]ConversationEntry, error)
	SavePromptFeedback(ctx context.Context, userID uuid.UUID, original, edited, prospectContext, channel string) error
	GetRecentFeedback(ctx context.Context, userID uuid.UUID, limit int) ([]PromptFeedback, error)
}

// Repository composes all sub-repositories for the sequences bounded context.
type Repository interface {
	SequenceRepo
	StepRepo
	OutboundMessageRepo
}

// AIMessageGenerator generates outbound messages using AI.
type AIMessageGenerator interface {
	GenerateColdMessage(ctx context.Context, name, title, company, prospectContext, stepHint, previousMessage, source, feedbackExamples string) (string, error)
	GenerateTelegramMessage(ctx context.Context, name, title, company, prospectContext, stepHint, previousMessage, source, feedbackExamples string) (string, error)
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
	WhatsApp         string
	TelegramUsername string
	Context          string
	Source           string
	SourceID         *uuid.UUID
	Status           string
	VerifyStatus     string
	VerifiedAt       *time.Time
}

// CanReceiveSequence returns true if the prospect is eligible for sequence messages.
func (p *ProspectView) CanReceiveSequence() bool {
	if p.Status == ProspectStatusConverted || p.Status == ProspectStatusOptedOut || p.Status == ProspectStatusInSequence || p.Status == ProspectStatusReplied {
		return false
	}
	if p.VerifyStatus == VerifyStatusInvalid {
		return false
	}
	if p.VerifyStatus == VerifyStatusNotChecked && p.Email != "" {
		return false
	}
	return true
}

// Prospect status values used by the sequences context.
const (
	ProspectStatusInSequence = "in_sequence"
	ProspectStatusReplied    = "replied"
	ProspectStatusConverted  = "converted"
	ProspectStatusOptedOut   = "opted_out"

	VerifyStatusInvalid    = "invalid"
	VerifyStatusNotChecked = "not_checked"
)

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
