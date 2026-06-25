package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrEmailNotConfigured is returned by an EmailConfigChecker when neither
// Resend nor SMTP is configured for the user. Sentinel so the handler can
// errors.Is it and surface a 400 with a human cause + remedy (instead of the
// async sender dropping the message silently).
var ErrEmailNotConfigured = errors.New("email not configured")

// ErrProspectNotOwned is returned by Launch when a requested prospect does not
// belong to the authenticated caller. Sentinel so the handler can errors.Is it
// and answer 404 (not leaking whether the prospect exists for another tenant).
var ErrProspectNotOwned = errors.New("prospect not owned by caller")

// ErrSequenceNotOwned is returned when a requested sequence — or a step or
// launch targeting it — does not belong to the authenticated caller, or does
// not exist. A missing and a foreign sequence return the SAME sentinel so the
// caller can't tell them apart (anti-enumeration); the handler errors.Is it
// and answers 404.
var ErrSequenceNotOwned = errors.New("sequence not owned by caller")

// ErrMessageNotOwned is returned when an outbound message operation (approve,
// reject, edit) targets a message whose prospect does not belong to the
// authenticated caller, or that does not exist. Missing and foreign collapse to
// the same sentinel (anti-enumeration) → 404 at the handler.
var ErrMessageNotOwned = errors.New("outbound message not owned by caller")

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

// EmailConfigChecker reports whether outbound email (Resend or SMTP) is
// configured for a user. Implemented by an adapter in the composition root
// over the settings store + env fallback; lets launch preflight email steps
// before queuing messages the async sender would otherwise drop silently.
// Returns nil when configured, ErrEmailNotConfigured otherwise.
type EmailConfigChecker interface {
	IsEmailConfigured(ctx context.Context, userID uuid.UUID) error
}

// AutopilotSettings is the resolved autopilot configuration for one launch.
type AutopilotSettings struct {
	// Enabled reports whether launch should auto-approve queued messages
	// (skipping manual approval) so the async sender dispatches them.
	Enabled bool
	// SendDelay is the grace window an auto-approved message waits after launch
	// before the sender may pick it up — time for the operator to still
	// intervene. Zero means send at the next sender tick.
	SendDelay time.Duration
}

// AutopilotChecker resolves a user's autopilot configuration — automatic
// approval and sending of the messages a sequence launch queues. Implemented
// by an adapter in the composition root over the settings store (the AutoSend
// flag + send-delay). When enabled, launch promotes each queued message
// straight to Approved so the async sender dispatches it without a manual
// approval step; when disabled (the default), messages stay Draft and wait for
// a human.
//
// A returned error fails the launch: the usecase never guesses the send mode,
// so an unreadable setting can never silently auto-send real messages. A nil
// checker (the default wiring) means autopilot is off.
type AutopilotChecker interface {
	ResolveAutopilot(ctx context.Context, userID uuid.UUID) (AutopilotSettings, error)
}
