package inbox

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// --- Lead status (inbox-local) ---

type LeadStatus string

const (
	StatusNew       LeadStatus = "new"
	StatusQualified LeadStatus = "qualified"
)

// --- Channel (inbox-local) ---

type Channel string

const (
	ChannelTelegram Channel = "telegram"
	ChannelEmail    Channel = "email"
)

// --- Message direction (inbox-local) ---

type MessageDirection string

const (
	DirectionInbound  MessageDirection = "inbound"
	DirectionOutbound MessageDirection = "outbound"
)

// --- Read models ---

// InboxLead is the inbox-local read model for a lead.
type InboxLead struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Channel        Channel
	ContactName    string
	Company        string
	FirstMessage   string
	Status         LeadStatus
	TelegramChatID *int64
	EmailAddress   *string
	SourceID       *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// InboxMessage is the inbox-local read model for a message.
type InboxMessage struct {
	ID        uuid.UUID
	LeadID    uuid.UUID
	Direction MessageDirection
	Body      string
	SentAt    time.Time
}

// InboxQualification is the inbox-local read model for a qualification.
type InboxQualification struct {
	ID                uuid.UUID
	LeadID            uuid.UUID
	IdentifiedNeed    string
	EstimatedBudget   string
	Deadline          string
	Score             int
	ScoreReason       string
	RecommendedAction string
	ProviderUsed      string
	GeneratedAt       time.Time
}

// QualificationResult is the inbox-local model for AI qualification output.
type QualificationResult struct {
	IdentifiedNeed    string
	EstimatedBudget   string
	Deadline          string
	Score             int
	ScoreReason       string
	RecommendedAction string
}

// InboxConfig holds only the fields inbox needs from user configuration.
type InboxConfig struct {
	IMAPHost         string
	IMAPPort         string
	IMAPUser         string
	IMAPPassword     string
	TelegramBotToken string
}

// --- Ports ---

// LeadRepository is the interface inbox needs from leads.
type LeadRepository interface {
	GetLeadByTelegramChatID(ctx context.Context, userID uuid.UUID, chatID int64) (*InboxLead, error)
	GetLeadByEmailAddress(ctx context.Context, userID uuid.UUID, email string) (*InboxLead, error)
	CreateLead(ctx context.Context, lead *InboxLead) error
	UpdateFirstMessage(ctx context.Context, id uuid.UUID, message string) error
	CreateMessage(ctx context.Context, msg *InboxMessage) error
	UpsertQualification(ctx context.Context, q *InboxQualification) error
	UpdateLeadStatus(ctx context.Context, id uuid.UUID, status LeadStatus) error
}

// ProspectMatch is a port-level read model for prospect data used by inbox.
type ProspectMatch struct {
	ID       uuid.UUID
	Name     string
	Company  string
	SourceID *uuid.UUID
	Status   string
}

const ProspectStatusConverted = "converted"

// ProspectRepository is the interface inbox needs from prospects.
type ProspectRepository interface {
	FindByEmail(ctx context.Context, userID uuid.UUID, email string) (*ProspectMatch, error)
	FindByTelegramUsername(ctx context.Context, userID uuid.UUID, username string) (*ProspectMatch, error)
	ConvertToLead(ctx context.Context, prospectID, leadID uuid.UUID) error
}

// SequenceRepository is the interface inbox needs from sequences.
type SequenceRepository interface {
	MarkRepliedByProspect(ctx context.Context, prospectID uuid.UUID) error
}

// AIQualifier qualifies leads using AI.
type AIQualifier interface {
	Qualify(ctx context.Context, contactName, channel, firstMessage string) (*QualificationResult, error)
	ProviderName() string
}

// ConfigStore reads user configuration.
type ConfigStore interface {
	GetConfig(ctx context.Context, userID uuid.UUID) (*InboxConfig, error)
}

// IdentityLinker is the narrow port inbox needs from the leads-context
// identity machinery: take a freshly created lead and a tuple of raw
// identifiers, resolve them to a unified Identity (creating one if
// none matches), and link the lead to it. The adapter in the
// composition root bridges this to leads.IdentityResolver + leads.
// IdentityRepository.LinkLead.
//
// Implementations MUST be idempotent and tolerate partially-empty
// identifier tuples — inbox supplies only the channel-native handle
// (email for email-poller, telegram_username for telegram-bot).
type IdentityLinker interface {
	LinkLeadToIdentity(ctx context.Context, userID, leadID uuid.UUID, email, phone, telegramUsername string) error
}

// PendingReplyRepository persists the HITL approval queue. Every read
// method is scoped by userID — the repository never returns a row that
// belongs to another tenant, so an attacker who guesses or enumerates
// IDs still gets nil/empty. Callers SHOULD treat nil-without-error as
// "not found OR not owned" and answer 404 to keep the two
// indistinguishable on the wire.
type PendingReplyRepository interface {
	Save(ctx context.Context, pr *PendingReply) error
	GetByID(ctx context.Context, userID, id uuid.UUID) (*PendingReply, error)
	ListByLead(ctx context.Context, userID, leadID uuid.UUID) ([]*PendingReply, error)
	Update(ctx context.Context, pr *PendingReply) error
}
