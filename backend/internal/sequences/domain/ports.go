package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SequenceRepo manages sequence CRUD.
// Note: ToggleActive was removed — the usecase now loads the entity, calls
// Activate/Deactivate, and persists via UpdateSequence. This keeps the port
// surface thinner and routes all state changes through the domain.
type SequenceRepo interface {
	ListSequences(ctx context.Context, userID uuid.UUID) ([]Sequence, error)
	GetSequence(ctx context.Context, id uuid.UUID) (*Sequence, error)
	CreateSequence(ctx context.Context, s *Sequence) error
	UpdateSequence(ctx context.Context, s *Sequence) error
	DeleteSequence(ctx context.Context, id uuid.UUID) error
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
	// MarkSent persists the sent state + sent_at timestamp computed by the
	// caller (typically via OutboundMessage.MarkSent on the domain entity).
	// The repository MUST write sentAt verbatim — do NOT fall back to
	// server-side NOW() — so the authoritative clock lives at the
	// orchestration layer, not in SQL.
	MarkSent(ctx context.Context, id uuid.UUID, sentAt time.Time) error
	// MarkBounced persists the bounced terminal state. bouncedAt is accepted
	// so the port's contract is clock-injection-ready (symmetric with
	// MarkSent); implementations may discard the value until the schema
	// grows a bounced_at column, but the port MUST NOT fall back to SQL
	// NOW() — the entity already owns the clock via OutboundMessage.MarkBounced.
	MarkBounced(ctx context.Context, id uuid.UUID, bouncedAt time.Time) error
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
//
// Notably, ProspectView carries the pre-computed IsEligibleForSequence flag
// rather than re-implementing the eligibility rule. The prospects context
// owns that rule (Prospect.CanLaunchSequence); the adapter copies the
// decision onto the view. This keeps the business rule in one place — any
// edit to the predicate cannot drift between contexts.
type ProspectView struct {
	ID                    uuid.UUID
	UserID                uuid.UUID
	Name                  string
	Company               string
	Title                 string
	Email                 string
	Phone                 string
	WhatsApp              string
	TelegramUsername      string
	Context               string
	Source                string
	SourceID              *uuid.UUID
	Status                string
	VerifyStatus          string
	VerifiedAt            *time.Time
	IsEligibleForSequence bool // computed from prospects.Prospect.CanLaunchSequence
}

// ProspectReader provides read access to prospect data and the narrow set of
// status transitions sequences is allowed to request.
//
// Previously this port had a generic UpdateStatus(ctx, id, status string)
// method, which forced sequences to carry its own string constants mirroring
// the prospects enum — a bounded-context duplication smell. The named
// transitions below eliminate the duplication: sequences never names a
// prospect-status value directly; the adapter in the composition root knows
// the enum and calls Prospect.TransitionTo.
type ProspectReader interface {
	GetProspect(ctx context.Context, id uuid.UUID) (*ProspectView, error)
	// MarkInSequence transitions a prospect into the in_sequence state. Fails
	// if the current state forbids it (e.g. already converted).
	MarkInSequence(ctx context.Context, id uuid.UUID) error
	// MarkConverted transitions a prospect into the converted terminal state.
	// Fails if the current state forbids it (e.g. opted_out).
	MarkConverted(ctx context.Context, id uuid.UUID) error
}

// LeadCreator creates leads from converted prospects.
type LeadCreator interface {
	CreateLeadFromProspect(ctx context.Context, prospect *ProspectView, userID uuid.UUID) (uuid.UUID, error)
}

// TxManager provides transactional execution.
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
