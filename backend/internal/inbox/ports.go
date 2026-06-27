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

// LeadCreatedObserver is notified after a new inbound lead is created (Telegram
// or Email), so cross-context side-effects (e.g. emitting a lead.created
// webhook, #181) can fire without the inbox context importing those modules.
// Implemented in the composition root; nil disables the hook; the method
// returns nothing — a failing side-effect must never fail lead intake.
type LeadCreatedObserver interface {
	OnLeadCreated(ctx context.Context, lead *InboxLead)
}

// LeadQualifiedObserver is notified after the inbox auto-qualification path
// scores an inbound lead and persists status=qualified. Without this, a
// lead.qualified webhook would only fire on the manual /qualify API path (which
// goes through leads.UseCase) and miss the dominant automatic flow. Same
// contract as the other inbox observers: composition-root impl, nil disables,
// returns nothing (a failing side-effect must not fail qualification).
type LeadQualifiedObserver interface {
	OnLeadQualified(ctx context.Context, lead *InboxLead)
}

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
	ArchivedAt     *time.Time
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
	// UnarchiveLead resurfaces an archived lead when its contact re-engages
	// (new inbound message). Idempotent: a no-op (returns nil) when the lead
	// is already active, so callers can invoke it on the re-engagement path
	// without first checking state.
	UnarchiveLead(ctx context.Context, id uuid.UUID) error
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

// EnrichmentEnqueuer is the narrow port inbox needs from the enrichment
// context: enqueue a best-effort background company-data lookup for a newly
// created lead's email. Implementations must tolerate a free/personal or empty
// email (a no-op); errors are logged, never block the inbound flow.
type EnrichmentEnqueuer interface {
	Enqueue(ctx context.Context, userID uuid.UUID, email string) error
}

// ReplyTarget is the inbox-local read model carrying the channel-native
// destination for an approved reply: the lead's Telegram chat id and/or
// email address. Neither value is derivable from a PendingReply alone —
// both live on the lead row — so the reply dispatchers resolve a target by
// lead id at send time. The composition-root adapter maps a leads-context
// Lead onto this struct so the inbox dispatchers never import the leads
// domain.
//
// Both fields are optional: a Telegram-only lead has a nil EmailAddress and
// vice versa. The channel-specific dispatcher validates the one field it
// needs and returns an error when that destination is absent, so a
// misrouted reply never silently no-ops.
type ReplyTarget struct {
	TelegramChatID *int64
	EmailAddress   *string
}

// ReplyTargetLookup resolves the channel destination for an approved reply
// by lead id. Implementations live in the composition root (an adapter over
// the leads repository). A (nil, nil) return signals "lead not found" so the
// dispatcher can surface a clear error without the inbox context learning
// the leads domain's not-found semantics; a non-nil error is a genuine
// lookup failure the dispatcher propagates so the usecase keeps the row
// Approved for retry.
type ReplyTargetLookup interface {
	LookupReplyTarget(ctx context.Context, leadID uuid.UUID) (*ReplyTarget, error)
}

// EmailSender is the narrow port inbox needs to dispatch an
// approved PendingReply on the email channel. Implementations live
// in the composition root (an adapter that wraps the outbound
// package's SMTP/Resend logic) so the inbox package stays free of
// transport-layer imports.
//
// SendEmail MUST resolve the user's SMTP/Resend configuration per
// call — the same row of pending_replies can outlive a config
// change. Returning an error keeps the entity in Approved status so
// the operator can retry; the usecase will NOT mark it sent.
type EmailSender interface {
	SendEmail(ctx context.Context, userID uuid.UUID, to, subject, body string) error
}

// PendingReplyProposer enqueues an auto-drafted reply for human
// approval. It is the inversion-of-control seam between the inbox
// pollers (Telegram bot, future email auto-replies) and the HITL
// usecase: the poller knows only the abstract action "park this draft
// for the operator", never how the queue is persisted.
//
// Implementations MUST be safe to call from background goroutines and
// MUST NOT block on dispatch — actual delivery happens after operator
// approval, not at Propose time.
//
// inboundText is the untrusted message that triggered the reply (the
// lead's words), kept distinct from body (the outbound draft): the
// usecase classifies inboundText to stamp the reply's InputSeverity, so
// a reply provoked by a Block-flagged payload can be refused at dispatch.
type PendingReplyProposer interface {
	Propose(ctx context.Context, userID, leadID uuid.UUID, channel Channel, kind PendingReplyKind, body, inboundText string) (*PendingReply, error)
}

// InputClassifier returns the firewall severity of an inbound message.
// It is the inbox-side port for the security InputFirewall; the concrete
// adapter (composition root) maps the security verdict onto inbox.Severity
// so the inbox context never imports internal/ai/security directly.
type InputClassifier interface {
	Classify(text string) Severity
}

// PendingReplyRepository persists the HITL approval queue. Every read
// method is scoped by userID — the repository never returns a row that
// belongs to another tenant, so an attacker who guesses or enumerates
// IDs still gets nil/empty. Callers SHOULD treat nil-without-error as
// "not found OR not owned" and answer 404 to keep the two
// indistinguishable on the wire.
type PendingReplyRepository interface {
	// Save persists a new PendingReply. Returns
	// ErrPendingReplyDuplicatePending if a row with the same
	// (user_id, lead_id, kind, body) tuple already exists in the
	// pending status — the partial unique dedup index catches the
	// Telegram-reconnect duplicate-Propose case at the DB level.
	Save(ctx context.Context, pr *PendingReply) error
	GetByID(ctx context.Context, userID, id uuid.UUID) (*PendingReply, error)
	ListByLead(ctx context.Context, userID, leadID uuid.UUID) ([]*PendingReply, error)
	// FindPendingByContent returns the existing pending row whose
	// (user_id, lead_id, kind, body) tuple matches, or nil if none.
	// Used by the usecase after Save returns
	// ErrPendingReplyDuplicatePending to surface the previously-
	// enqueued entity to the caller. body is matched against the
	// trimmed value the factory stored.
	FindPendingByContent(ctx context.Context, userID, leadID uuid.UUID, kind PendingReplyKind, body string) (*PendingReply, error)
	// CountPendingByUser returns the number of pending rows per lead
	// for the given user. Leads with no pending rows are absent from
	// the map (callers default to zero). Used by the leads-context
	// inbox-list badge — wired through an adapter so the leads package
	// stays free of inbox-package imports.
	CountPendingByUser(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]int, error)
	// ListPendingByUser returns every pending-status row for the user,
	// joined with the minimum lead context the operator queue needs to
	// render contact + company without an N+1 fetch on the frontend.
	// Scoped by user_id; cross-tenant rows are silently filtered, never
	// surfaced. Rows are ordered by created_at DESC so the newest draft
	// floats to the top of the queue — matches the outbound-queue
	// convention. Returns an empty slice (not nil) when nothing is
	// pending so callers can iterate without a nil-check.
	ListPendingByUser(ctx context.Context, userID uuid.UUID) ([]*PendingReplyWithLead, error)
	// Update writes pr only when the persisted row still matches the
	// expectedStatus the caller observed at load time — an optimistic
	// lock that prevents two operators from concurrently approving
	// the same pending reply (and therefore double-firing the
	// dispatcher). On mismatch (or missing/cross-tenant row) the
	// caller receives ErrPendingReplyNotFound; the usecase layer maps
	// this to ErrPendingReplyAlreadyDecided when the row was loaded
	// in the same operation.
	Update(ctx context.Context, pr *PendingReply, expectedStatus PendingReplyStatus) error
	// UpdateBody persists a body-only edit on a pending row, scoped by
	// user_id and optimistic-locked on expectedStatus (always Pending
	// at the call site — Approved/Sent/Rejected rows are immutable per
	// the domain invariant). Other columns are untouched. Missing /
	// cross-tenant / lock-violated rows return ErrPendingReplyNotFound;
	// the usecase maps that to ErrPendingReplyAlreadyDecided when the
	// row was loaded inside the same operation.
	UpdateBody(ctx context.Context, pr *PendingReply, expectedStatus PendingReplyStatus) error
}
