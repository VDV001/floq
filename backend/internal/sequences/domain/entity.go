package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// --- OutboundStatus value object ---

type OutboundStatus string

const (
	OutboundStatusDraft    OutboundStatus = "draft"
	OutboundStatusApproved OutboundStatus = "approved"
	OutboundStatusSent     OutboundStatus = "sent"
	OutboundStatusRejected OutboundStatus = "rejected"
	OutboundStatusBounced  OutboundStatus = "bounced"
)

// IsValid returns true if the OutboundStatus is one of the known values.
func (s OutboundStatus) IsValid() bool {
	switch s {
	case OutboundStatusDraft, OutboundStatusApproved, OutboundStatusSent, OutboundStatusRejected, OutboundStatusBounced:
		return true
	default:
		return false
	}
}

// String returns the string representation of the OutboundStatus.
func (s OutboundStatus) String() string {
	return string(s)
}

// outboundTransitions encodes the business-legal state machine for outbound
// messages. The operator approves or rejects a draft; an approved message
// gets sent (delivery succeeded) or bounced (delivery rejected synchronously
// at the SMTP/API layer); a sent message may later be marked bounced after
// an async delivery-failure notification. Terminal states (rejected,
// bounced) have no outgoing edges.
//
// Note: `approved → bounced` covers synchronous SMTP/API rejections at send
// time (e.g. 550 mailbox unknown). `sent → bounced` covers asynchronous
// bounces delivered later. Both ultimately mean "undeliverable" — different
// paths to the same terminal state.
var outboundTransitions = map[OutboundStatus][]OutboundStatus{
	OutboundStatusDraft:    {OutboundStatusApproved, OutboundStatusRejected},
	OutboundStatusApproved: {OutboundStatusSent, OutboundStatusRejected, OutboundStatusBounced},
	OutboundStatusSent:     {OutboundStatusBounced},
}

// CanTransitionTo reports whether target is a legal next state for s.
func (s OutboundStatus) CanTransitionTo(target OutboundStatus) bool {
	for _, allowed := range outboundTransitions[s] {
		if allowed == target {
			return true
		}
	}
	return false
}

// --- StepChannel value object ---

type StepChannel string

const (
	StepChannelEmail     StepChannel = "email"
	StepChannelTelegram  StepChannel = "telegram"
	StepChannelPhoneCall StepChannel = "phone_call"
)

// IsValid returns true if the StepChannel is one of the known values.
func (c StepChannel) IsValid() bool {
	switch c {
	case StepChannelEmail, StepChannelTelegram, StepChannelPhoneCall:
		return true
	default:
		return false
	}
}

// String returns the string representation of the StepChannel.
func (c StepChannel) String() string {
	return string(c)
}

// --- Domain entities ---

type Sequence struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	IsActive  bool
	CreatedAt time.Time
}

// NewSequence creates a new Sequence with generated ID and timestamp.
// Returns an error if required invariants are violated:
//   - userID must not be the zero UUID;
//   - name must be non-empty after trimming.
//
// Sequences start inactive — the operator must explicitly Toggle to enable
// outbound sends, which is a deliberate safeguard against accidental blast.
func NewSequence(userID uuid.UUID, name string) (*Sequence, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("sequence userID is required")
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("sequence name is required")
	}
	return &Sequence{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      name,
		IsActive:  false,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// Rename changes the sequence's name, enforcing the non-empty invariant.
// Callers persist via Repository.UpdateSequence after a successful rename.
func (s *Sequence) Rename(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("sequence name is required")
	}
	s.Name = name
	return nil
}

// Activate/Deactivate explicitly model the toggle rather than leaving IsActive
// as a public bool — future rules (e.g. "can't activate a sequence with no
// steps") can live on these methods without callers changing.
func (s *Sequence) Activate()   { s.IsActive = true }
func (s *Sequence) Deactivate() { s.IsActive = false }

type SequenceStep struct {
	ID         uuid.UUID
	SequenceID uuid.UUID
	StepOrder  int
	DelayDays  int
	PromptHint string
	Channel    StepChannel
	CreatedAt  time.Time
}

// NewSequenceStep creates a new SequenceStep with generated ID and timestamp.
func NewSequenceStep(sequenceID uuid.UUID, stepOrder, delayDays int, channel StepChannel, hint string) *SequenceStep {
	return &SequenceStep{
		ID:         uuid.New(),
		SequenceID: sequenceID,
		StepOrder:  stepOrder,
		DelayDays:  delayDays,
		PromptHint: hint,
		Channel:    channel,
		CreatedAt:  time.Now().UTC(),
	}
}

type OutboundMessage struct {
	ID          uuid.UUID
	ProspectID  uuid.UUID
	SequenceID  uuid.UUID
	StepOrder   int
	Channel     StepChannel
	Body        string
	Status      OutboundStatus
	ScheduledAt time.Time
	SentAt      *time.Time
	BouncedAt   *time.Time // populated by MarkBounced; persisted once schema adds the column
	CreatedAt   time.Time
}

// TransitionTo validates and applies a status change, rejecting transitions
// that violate the outbound state machine (see outboundTransitions). Returns
// a descriptive error when the caller attempts to skip a step or reopen a
// terminal state (rejected/bounced).
func (m *OutboundMessage) TransitionTo(target OutboundStatus) error {
	if !target.IsValid() {
		return fmt.Errorf("invalid outbound status: %q", target)
	}
	if !m.Status.CanTransitionTo(target) {
		return fmt.Errorf("cannot transition outbound message from %q to %q", m.Status, target)
	}
	m.Status = target
	return nil
}

// MarkSent applies the Approved → Sent transition and records the send time.
// Returns an error if the message is not currently approved.
func (m *OutboundMessage) MarkSent(sentAt time.Time) error {
	if err := m.TransitionTo(OutboundStatusSent); err != nil {
		return err
	}
	m.SentAt = &sentAt
	return nil
}

// MarkBounced applies a transition into the Bounced terminal state and
// records the bounce time. Valid from Approved (synchronous SMTP reject at
// send time) or Sent (asynchronous bounce after delivery); see
// outboundTransitions. The entity owns the clock symmetrically with MarkSent
// so that when bounced_at becomes a persisted column, the repository
// contract can already accept it (see sequences.Repository.MarkBounced).
func (m *OutboundMessage) MarkBounced(bouncedAt time.Time) error {
	if err := m.TransitionTo(OutboundStatusBounced); err != nil {
		return err
	}
	m.BouncedAt = &bouncedAt
	return nil
}

// NewOutboundMessage creates a new OutboundMessage in draft status with generated ID.
func NewOutboundMessage(prospectID, sequenceID uuid.UUID, stepOrder int, channel StepChannel, body string, scheduledAt time.Time) *OutboundMessage {
	return &OutboundMessage{
		ID:          uuid.New(),
		ProspectID:  prospectID,
		SequenceID:  sequenceID,
		StepOrder:   stepOrder,
		Channel:     channel,
		Body:        body,
		Status:      OutboundStatusDraft,
		ScheduledAt: scheduledAt,
		CreatedAt:   time.Now().UTC(),
	}
}

type OutboundMessageWithProspect struct {
	OutboundMessage
	ProspectName  string
	ProspectEmail string
	Company       string
}

type Stats struct {
	Draft    int
	Approved int
	Sent     int
	Opened   int
	Replied  int
	Bounced  int
}

type ConversationEntry struct {
	Body   string
	Status string
	SentAt time.Time
}

type PromptFeedback struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	OriginalBody    string
	EditedBody      string
	ProspectContext string
	Channel         string
	CreatedAt       time.Time
}
