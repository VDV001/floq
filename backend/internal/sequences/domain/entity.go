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

// IsPendingDispatch reports whether a message in this status may still be sent
// — it is awaiting dispatch (a draft awaiting approval, or an approved message
// awaiting the send tick). Sent, rejected, and bounced messages will not be
// dispatched again: a later async sent→bounced is a delivery outcome, not a new
// send. A sequence run with no pending-dispatch messages left for a prospect has
// therefore finished sending — the basis for the sequence.completed event.
func (s OutboundStatus) IsPendingDispatch() bool {
	switch s {
	case OutboundStatusDraft, OutboundStatusApproved:
		return true
	default:
		return false
	}
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
	ID       uuid.UUID
	UserID   uuid.UUID
	Name     string
	IsActive bool
	// RequireApproval is the per-sequence outbound HITL gate. When true, every
	// message this sequence launches starts as a draft awaiting operator
	// approval — even under autopilot, which it overrides (see
	// InitialOutboundStatus). Default false keeps the prior behaviour: autopilot
	// (a user-global setting) alone decides whether a launch auto-sends.
	RequireApproval bool
	CreatedAt       time.Time
}

// InitialOutboundStatus decides the status a freshly launched message starts
// in. A message skips human review (starts Approved, so the async sender
// dispatches it) ONLY when autopilot is enabled AND the sequence does not
// require approval; otherwise it starts as a Draft awaiting an operator
// decision. requireApproval is the per-sequence HITL gate and overrides
// autopilot — a cautious sequence is always reviewed before send.
func InitialOutboundStatus(autopilotEnabled, requireApproval bool) OutboundStatus {
	if autopilotEnabled && !requireApproval {
		return OutboundStatusApproved
	}
	return OutboundStatusDraft
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
	// Body, when non-empty, is the manually written message used verbatim at
	// launch (no AI generation). PromptHint drives AI generation when Body is
	// empty. IsManual reports which mode this step is in.
	Body      string
	Channel   StepChannel
	CreatedAt time.Time
}

// IsManual reports whether the step carries a hand-written body that should be
// used verbatim instead of generating with AI.
func (s *SequenceStep) IsManual() bool {
	return s.Body != ""
}

// IsEmail reports whether the step is delivered over email. An empty channel
// counts as email: that matches launch's generation default (the AI switch
// treats "" as email) and the persisted channel, which the step handler
// normalizes empty -> "email" before saving. Used to scope the launch
// email-config preflight.
func (s *SequenceStep) IsEmail() bool {
	return s.Channel == StepChannelEmail || s.Channel == ""
}

// NewSequenceStep creates a new SequenceStep with generated ID and timestamp.
// A non-empty body marks the step as manual (used verbatim, no AI). The body is
// trimmed so a whitespace-only value can't masquerade as a manual step and ship
// an almost-empty message — keeping IsManual reliable at every entry point.
func NewSequenceStep(sequenceID uuid.UUID, stepOrder, delayDays int, channel StepChannel, hint, body string) *SequenceStep {
	return &SequenceStep{
		ID:         uuid.New(),
		SequenceID: sequenceID,
		StepOrder:  stepOrder,
		DelayDays:  delayDays,
		PromptHint: hint,
		Body:       strings.TrimSpace(body),
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
