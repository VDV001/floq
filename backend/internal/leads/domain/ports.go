package domain

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// --- Sentinel errors (mapped to 404 at HTTP boundary) ---

var (
	// ErrLeadNotFound is returned when a lead does not exist, or when the
	// caller's user_id does not match the lead's owner. The two cases are
	// deliberately indistinguishable to avoid leaking lead existence across
	// tenants via error responses.
	ErrLeadNotFound = errors.New("lead not found")

	// ErrProspectNotFound is returned when a prospect does not exist, or when
	// the caller's user_id does not match the prospect's owner.
	ErrProspectNotFound = errors.New("prospect not found")
)

// Repository defines persistence operations for the leads domain.
//
// All methods accept context.Context and are transaction-aware via
// db.ConnFromCtx — invoking any method within a db.TxManager.WithTx block
// runs the SQL against that transaction automatically.
type Repository interface {
	ListLeads(ctx context.Context, userID uuid.UUID) ([]LeadWithSource, error)
	GetLead(ctx context.Context, id uuid.UUID) (*Lead, error)
	// GetLeadForUser returns the lead iff it belongs to userID; returns nil
	// otherwise (ownership mismatch indistinguishable from not-found at this
	// layer — callers translate nil to domain.ErrLeadNotFound).
	GetLeadForUser(ctx context.Context, userID, leadID uuid.UUID) (*Lead, error)
	CreateLead(ctx context.Context, lead *Lead) error
	UpdateFirstMessage(ctx context.Context, id uuid.UUID, message string) error
	UpdateLeadStatus(ctx context.Context, id uuid.UUID, status LeadStatus) error
	UpdateSourceID(ctx context.Context, id uuid.UUID, sourceID *uuid.UUID) error
	GetLeadByTelegramChatID(ctx context.Context, userID uuid.UUID, chatID int64) (*Lead, error)
	GetLeadByEmailAddress(ctx context.Context, userID uuid.UUID, email string) (*Lead, error)
	StaleLeadsWithoutReminder(ctx context.Context, staleDays int) ([]Lead, error)

	ListMessages(ctx context.Context, leadID uuid.UUID) ([]Message, error)
	CreateMessage(ctx context.Context, msg *Message) error

	GetQualification(ctx context.Context, leadID uuid.UUID) (*Qualification, error)
	UpsertQualification(ctx context.Context, q *Qualification) error

	GetLatestDraft(ctx context.Context, leadID uuid.UUID) (*Draft, error)
	CreateDraft(ctx context.Context, d *Draft) error

	CreateReminder(ctx context.Context, leadID uuid.UUID, message string) error

	CountMonthLeads(ctx context.Context, userID uuid.UUID) (int, error)
	CountTotalLeads(ctx context.Context, userID uuid.UUID) (int, error)
}

// MessageSender sends messages to leads via their channel.
type MessageSender interface {
	SendMessage(ctx context.Context, lead *Lead, body string) error
}

// AIService provides AI-powered operations for leads.
type AIService interface {
	Qualify(ctx context.Context, contactName string, channel Channel, firstMessage string) (*Qualification, error)
	DraftReply(ctx context.Context, contactName string, firstMessage string) (string, error)
	GenerateFollowup(ctx context.Context, contactName string, company string, days int) (string, error)
}

// Notifier sends alerts and notifications.
type Notifier interface {
	SendAlert(ctx context.Context, leadName string, company string, message string) error
}

// --- Prospect suggestion (cross-channel dedup) ---

// SuggestionConfidence classifies how strong the cross-channel match signal is.
type SuggestionConfidence string

const (
	ConfidenceHigh   SuggestionConfidence = "high"   // name + company match
	ConfidenceMedium SuggestionConfidence = "medium" // name + shared email domain
	ConfidenceLow    SuggestionConfidence = "low"    // name match only
)

// IsValid reports whether the confidence is one of the known values.
func (c SuggestionConfidence) IsValid() bool {
	switch c {
	case ConfidenceHigh, ConfidenceMedium, ConfidenceLow:
		return true
	}
	return false
}

// ProspectSuggestion is a read model exposed to the leads context describing
// a candidate prospect that might be the same person as the lead, but on
// a different channel. Its shape is defined by leads' needs; the prospects
// context converts its own Prospect entity into this DTO via an adapter.
type ProspectSuggestion struct {
	ProspectID       uuid.UUID
	Name             string
	Company          string
	Email            string
	TelegramUsername string
	SourceName       string
	Status           string
	Confidence       SuggestionConfidence
}

// ProspectSuggestionFinder is the port through which the leads usecase queries
// cross-channel prospect matches and mutates the suggestion state (link/dismiss).
// Implementation lives in the composition root and bridges to the prospects
// context and the dismissals table.
//
// All methods take userID explicitly and MUST enforce that the referenced lead
// and prospect belong to that user — adapter implementations return the
// sentinel ErrLeadNotFound / ErrProspectNotFound on any ownership mismatch so
// cross-tenant IDs look indistinguishable from non-existent ones.
type ProspectSuggestionFinder interface {
	// FindForLead returns candidate prospect matches for the lead, ordered by
	// confidence (high → low). Returns ErrLeadNotFound if the lead does not
	// belong to userID. Returns an empty slice when the lead has no name.
	FindForLead(ctx context.Context, userID, leadID uuid.UUID) ([]ProspectSuggestion, error)

	// CountsForUser returns the number of non-dismissed suggestions per lead
	// for the given user. Only entries with count > 0 are included.
	CountsForUser(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]int, error)

	// LinkProspect atomically: (a) validates that both lead and prospect belong
	// to userID, (b) marks the prospect as converted to the lead, (c) copies the
	// prospect's source_id onto the lead when the lead has none. The inheritance
	// rule lives on Lead.InheritsSourceFrom; the adapter just orchestrates the
	// transaction.
	LinkProspect(ctx context.Context, userID, leadID, prospectID uuid.UUID) error

	// DismissSuggestion records that the user rejected this prospect as a match
	// for this lead, so it won't be re-suggested. Validates ownership of both
	// sides before insert.
	DismissSuggestion(ctx context.Context, userID, leadID, prospectID uuid.UUID) error
}
