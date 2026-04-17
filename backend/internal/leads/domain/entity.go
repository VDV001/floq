package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// --- LeadStatus value object ---

type LeadStatus string

const (
	StatusNew            LeadStatus = "new"
	StatusQualified      LeadStatus = "qualified"
	StatusInConversation LeadStatus = "in_conversation"
	StatusFollowup       LeadStatus = "followup"
	StatusClosed         LeadStatus = "closed"
	StatusWon            LeadStatus = "won"
)

var allowedTransitions = map[LeadStatus][]LeadStatus{
	StatusNew:            {StatusQualified, StatusClosed},
	StatusQualified:      {StatusInConversation, StatusFollowup, StatusClosed, StatusWon},
	StatusInConversation: {StatusFollowup, StatusClosed, StatusWon},
	StatusFollowup:       {StatusInConversation, StatusClosed, StatusWon},
	StatusWon:            {StatusClosed},
}

func (s LeadStatus) IsValid() bool {
	switch s {
	case StatusNew, StatusQualified, StatusInConversation, StatusFollowup, StatusClosed, StatusWon:
		return true
	}
	return false
}

func (s LeadStatus) String() string {
	return string(s)
}

func (s LeadStatus) CanTransitionTo(target LeadStatus) bool {
	for _, allowed := range allowedTransitions[s] {
		if allowed == target {
			return true
		}
	}
	return false
}

// --- Channel value object ---

type Channel string

const (
	ChannelTelegram Channel = "telegram"
	ChannelEmail    Channel = "email"
)

func (c Channel) IsValid() bool {
	switch c {
	case ChannelTelegram, ChannelEmail:
		return true
	}
	return false
}

// --- MessageDirection value object ---

type MessageDirection string

const (
	DirectionInbound  MessageDirection = "inbound"
	DirectionOutbound MessageDirection = "outbound"
)

// --- Domain entities ---

type Lead struct {
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

// LeadWithSource is a read model for list views: the lead plus its joined
// source_name projection. Kept out of Lead itself so the entity stays clean
// of persistence-driven projection fields (see sources.SourceStat for the
// same pattern). Only assembled by the repository for listing.
type LeadWithSource struct {
	Lead
	SourceName string
}

// NewLead creates a new Lead with generated ID, status=new, and timestamps.
func NewLead(userID uuid.UUID, channel Channel, contactName, company, firstMessage string, telegramChatID *int64, emailAddress *string) (*Lead, error) {
	if !channel.IsValid() {
		return nil, fmt.Errorf("invalid channel: %q", channel)
	}
	if contactName == "" {
		return nil, fmt.Errorf("contact name is required")
	}
	now := time.Now().UTC()
	return &Lead{
		ID:             uuid.New(),
		UserID:         userID,
		Channel:        channel,
		ContactName:    contactName,
		Company:        company,
		FirstMessage:   firstMessage,
		Status:         StatusNew,
		TelegramChatID: telegramChatID,
		EmailAddress:   emailAddress,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// InheritsSourceFrom expresses the business rule for source_id propagation
// when a lead is linked to a prospect (issue #6 manual cross-channel dedup):
// the lead keeps its own source_id if it has one, otherwise it inherits the
// prospect's source_id (which may itself be nil).
//
// Returns (newSource, changed) — the source_id the lead should carry after
// linking, and whether it differs from the lead's current value. Callers
// should only persist when changed == true; they must not rely on pointer
// equality of newSource with the lead's current SourceID.
//
// Rationale: a source manually assigned to a lead represents an explicit
// decision by the operator; auto-inheritance from a prospect must not
// overwrite that decision.
func (l *Lead) InheritsSourceFrom(prospectSourceID *uuid.UUID) (newSource *uuid.UUID, changed bool) {
	if l.SourceID != nil || prospectSourceID == nil {
		return l.SourceID, false
	}
	return prospectSourceID, true
}

// SetSource mutates the lead's source_id and bumps updated_at. The caller is
// responsible for persisting via Repository.UpdateSourceID — SetSource exists
// so mutations go through an entity method rather than ad-hoc struct writes.
func (l *Lead) SetSource(newSourceID *uuid.UUID) {
	l.SourceID = newSourceID
	l.UpdatedAt = time.Now().UTC()
}

// OnOutboundSent applies the domain rule that sending an outbound message
// auto-advances a qualified lead into the in_conversation state. Returns
// true iff the lead's status actually changed (the caller persists in that
// case). Any invalid transition is silently ignored — only the
// qualified→in_conversation edge is business-relevant here.
func (l *Lead) OnOutboundSent() (changed bool) {
	if l.Status != StatusQualified {
		return false
	}
	if err := l.TransitionTo(StatusInConversation); err != nil {
		return false
	}
	return true
}

// TransitionTo validates and applies a status transition.
func (l *Lead) TransitionTo(target LeadStatus) error {
	if !target.IsValid() {
		return fmt.Errorf("invalid lead status: %q", target)
	}
	if !l.Status.CanTransitionTo(target) {
		return fmt.Errorf("cannot transition lead from %q to %q", l.Status, target)
	}
	l.Status = target
	l.UpdatedAt = time.Now().UTC()
	return nil
}

type Message struct {
	ID        uuid.UUID
	LeadID    uuid.UUID
	Direction MessageDirection
	Body      string
	SentAt    time.Time
}

// NewMessage creates a new Message with generated ID and timestamp.
func NewMessage(leadID uuid.UUID, direction MessageDirection, body string) *Message {
	return &Message{
		ID:        uuid.New(),
		LeadID:    leadID,
		Direction: direction,
		Body:      body,
		SentAt:    time.Now().UTC(),
	}
}

type Qualification struct {
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

// Qualification scoring thresholds expressed as domain constants so the
// banding rule is declared in one place. These match the scoring guidance
// given to the AI qualifier (see ai.Qualify prompt).
const (
	QualificationHotThreshold  = 80 // >= 80 → hot lead, high priority
	QualificationWarmThreshold = 50 // 50–79 → warm, normal priority
	// anything below WarmThreshold is "cold" — deprioritized but not rejected
)

// NewQualification creates a Qualification with generated ID and timestamp.
// Score is clamped to [0,100] — upstream callers (AI, manual overrides) may
// produce out-of-range values on edge cases; the domain enforces the bound
// rather than trusting the caller.
func NewQualification(leadID uuid.UUID, need, budget, deadline string, score int, scoreReason, action, provider string) *Qualification {
	q := &Qualification{
		ID:                uuid.New(),
		LeadID:            leadID,
		IdentifiedNeed:    need,
		EstimatedBudget:   budget,
		Deadline:          deadline,
		Score:             score,
		ScoreReason:       scoreReason,
		RecommendedAction: action,
		ProviderUsed:      provider,
		GeneratedAt:       time.Now().UTC(),
	}
	q.ClampScore()
	return q
}

// ClampScore enforces the 0..100 invariant on the qualification score.
// Exposed separately so that, when the invariant is checked outside the
// standard NewQualification path (e.g. manual field assignment in tests),
// it can still be applied. Production rehydration should prefer
// RehydrateQualification instead — that factory runs the clamp at
// construction time, making bad scores unrepresentable.
func (q *Qualification) ClampScore() {
	if q.Score < 0 {
		q.Score = 0
	}
	if q.Score > 100 {
		q.Score = 100
	}
}

// RehydrateQualification reconstructs a Qualification from persisted or
// cross-context DTO values while still enforcing domain invariants. Unlike
// NewQualification, it accepts caller-supplied ID and GeneratedAt (because
// rehydration must preserve identity and original timestamp), but it
// applies ClampScore internally so no caller can persist an out-of-range
// score — the invariant lives on the factory, not on adapter diligence.
//
// Used by the composition root when translating inbox's InboxQualification
// into the leads domain (see cmd/server/adapters.go:fromInboxQualification).
func RehydrateQualification(id, leadID uuid.UUID, need, budget, deadline string, score int, scoreReason, action, provider string, generatedAt time.Time) *Qualification {
	q := &Qualification{
		ID:                id,
		LeadID:            leadID,
		IdentifiedNeed:    need,
		EstimatedBudget:   budget,
		Deadline:          deadline,
		Score:             score,
		ScoreReason:       scoreReason,
		RecommendedAction: action,
		ProviderUsed:      provider,
		GeneratedAt:       generatedAt,
	}
	q.ClampScore()
	return q
}

// IsHot reports whether the qualification score puts this lead into the
// top priority band. Encapsulates the threshold so callers don't hard-code
// magic numbers (and the business rule can move without breaking callers).
func (q *Qualification) IsHot() bool {
	return q.Score >= QualificationHotThreshold
}

// IsWarm reports whether the score is in the middle band (50–79). Mutually
// exclusive with IsHot.
func (q *Qualification) IsWarm() bool {
	return q.Score >= QualificationWarmThreshold && q.Score < QualificationHotThreshold
}

type Draft struct {
	ID        uuid.UUID
	LeadID    uuid.UUID
	Body      string
	CreatedAt time.Time
}

// NewDraft creates a Draft with generated ID and timestamp. Empty bodies are
// rejected — a draft with no body has no purpose and breaks downstream
// expectations (preview, send-as-is flow).
func NewDraft(leadID uuid.UUID, body string) (*Draft, error) {
	if body == "" {
		return nil, fmt.Errorf("draft body is required")
	}
	return &Draft{
		ID:        uuid.New(),
		LeadID:    leadID,
		Body:      body,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// IsEmpty reports whether the draft has any content to send. Useful for
// callers that may receive a persisted draft with externally-wiped body.
func (d *Draft) IsEmpty() bool {
	return d.Body == ""
}

type Reminder struct {
	ID        uuid.UUID
	LeadID    uuid.UUID
	Message   string
	CreatedAt time.Time
	Dismissed bool
}

// NewReminder constructs a Reminder with validated fields: leadID must be set,
// message must be non-empty. Reminders start non-dismissed.
func NewReminder(leadID uuid.UUID, message string) (*Reminder, error) {
	if leadID == uuid.Nil {
		return nil, fmt.Errorf("reminder leadID is required")
	}
	if message == "" {
		return nil, fmt.Errorf("reminder message is required")
	}
	return &Reminder{
		ID:        uuid.New(),
		LeadID:    leadID,
		Message:   message,
		CreatedAt: time.Now().UTC(),
		Dismissed: false,
	}, nil
}

// Dismiss marks the reminder as handled. Idempotent — dismissing an
// already-dismissed reminder is a no-op (avoids spurious errors in
// UI double-click scenarios).
func (r *Reminder) Dismiss() {
	r.Dismissed = true
}
