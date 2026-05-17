package inbox

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PendingReplyKind enumerates the categories of inbox-originated outbound
// replies that require human-in-the-loop approval before delivery. Each new
// auto-generated reply path that touches a customer-visible channel SHOULD
// add a new kind and route through the PendingReply workflow rather than
// sending directly.
type PendingReplyKind string

const (
	// PendingReplyKindBookingLink is an auto-drafted reply that contains a
	// calendar booking link, triggered when an inbound message looks like
	// the lead is ready to schedule a call. Misfires here would leak a
	// booking URL to a lead who never asked — hence the approval gate.
	PendingReplyKindBookingLink PendingReplyKind = "booking_link"
)

// IsValid reports whether the kind matches a known PendingReplyKind value.
func (k PendingReplyKind) IsValid() bool {
	switch k {
	case PendingReplyKindBookingLink:
		return true
	default:
		return false
	}
}

// String returns the underlying string for logging / persistence.
func (k PendingReplyKind) String() string { return string(k) }

// PendingReplyStatus tracks the lifecycle of a draft reply through the HITL
// approval workflow.
type PendingReplyStatus string

const (
	PendingReplyStatusPending  PendingReplyStatus = "pending"
	PendingReplyStatusApproved PendingReplyStatus = "approved"
	PendingReplyStatusSent     PendingReplyStatus = "sent"
	PendingReplyStatusRejected PendingReplyStatus = "rejected"
)

// IsValid reports whether the status is one of the known values.
func (s PendingReplyStatus) IsValid() bool {
	switch s {
	case PendingReplyStatusPending, PendingReplyStatusApproved, PendingReplyStatusSent, PendingReplyStatusRejected:
		return true
	default:
		return false
	}
}

// String returns the underlying string for logging / persistence.
func (s PendingReplyStatus) String() string { return string(s) }

// pendingReplyTransitions encodes the legal state machine. The operator
// approves or rejects a pending draft; an approved draft becomes sent once
// the dispatcher confirms delivery. Rejected and sent are terminal.
var pendingReplyTransitions = map[PendingReplyStatus][]PendingReplyStatus{
	PendingReplyStatusPending:  {PendingReplyStatusApproved, PendingReplyStatusRejected},
	PendingReplyStatusApproved: {PendingReplyStatusSent},
}

// CanTransitionTo reports whether target is a legal next state for s.
func (s PendingReplyStatus) CanTransitionTo(target PendingReplyStatus) bool {
	return slices.Contains(pendingReplyTransitions[s], target)
}

// --- Sentinels ---

var (
	ErrPendingReplyMissingUser      = errors.New("pending reply: user id is required")
	ErrPendingReplyMissingLead      = errors.New("pending reply: lead id is required")
	ErrPendingReplyInvalidChannel   = errors.New("pending reply: invalid channel")
	ErrPendingReplyInvalidKind      = errors.New("pending reply: invalid kind")
	ErrPendingReplyEmptyBody        = errors.New("pending reply: body is required")
	ErrPendingReplyInvalidTransition = errors.New("pending reply: invalid status transition")
	// ErrPendingReplyDuplicatePending signals that a Save attempt
	// collided with the partial-unique dedup index for an in-flight
	// pending row with identical (user_id, lead_id, kind, body). The
	// usecase translates this into a silent return of the previously-
	// enqueued entity so the caller treats re-Propose as idempotent.
	ErrPendingReplyDuplicatePending = errors.New("pending reply: duplicate pending row for same content")
)

// PendingReply is an inbox-originated outbound message awaiting human
// approval before being delivered to the lead. It is the aggregate root for
// the HITL (human-in-the-loop) gate that protects auto-drafted replies from
// reaching customers without explicit operator consent.
type PendingReply struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	LeadID    uuid.UUID
	Channel   Channel
	Kind      PendingReplyKind
	Body      string
	Status    PendingReplyStatus
	CreatedAt time.Time
	DecidedAt *time.Time
	SentAt    *time.Time
}

// TransitionTo validates and applies a status change, rejecting transitions
// that violate the PendingReply state machine. Callers should prefer the
// dedicated Approve / Reject / MarkSent methods, which also stamp the
// relevant timestamp atomically with the status change.
func (p *PendingReply) TransitionTo(target PendingReplyStatus) error {
	if !target.IsValid() {
		return fmt.Errorf("%w: unknown target %q", ErrPendingReplyInvalidTransition, target)
	}
	if !p.Status.CanTransitionTo(target) {
		return fmt.Errorf("%w: %q -> %q", ErrPendingReplyInvalidTransition, p.Status, target)
	}
	p.Status = target
	return nil
}

// Approve moves a pending reply into the Approved status and stamps
// DecidedAt. Use this when the operator confirms the draft should be sent;
// the actual delivery is a separate step (MarkSent) so that a failed send
// does not leave the entity in an inconsistent state.
func (p *PendingReply) Approve(at time.Time) error {
	if err := p.TransitionTo(PendingReplyStatusApproved); err != nil {
		return err
	}
	t := at
	p.DecidedAt = &t
	return nil
}

// Reject moves a pending reply into the terminal Rejected status and stamps
// DecidedAt. The draft body is preserved for audit / future reference.
func (p *PendingReply) Reject(at time.Time) error {
	if err := p.TransitionTo(PendingReplyStatusRejected); err != nil {
		return err
	}
	t := at
	p.DecidedAt = &t
	return nil
}

// MarkSent moves an approved reply into the terminal Sent status and
// records when delivery succeeded. Only valid from Approved.
func (p *PendingReply) MarkSent(at time.Time) error {
	if err := p.TransitionTo(PendingReplyStatusSent); err != nil {
		return err
	}
	t := at
	p.SentAt = &t
	return nil
}

// NewPendingReply constructs a PendingReply in the Pending status with a
// generated ID and timestamp, enforcing required invariants. Body is trimmed
// of surrounding whitespace; an entirely whitespace-only body is rejected.
func NewPendingReply(userID, leadID uuid.UUID, channel Channel, kind PendingReplyKind, body string) (*PendingReply, error) {
	if userID == uuid.Nil {
		return nil, ErrPendingReplyMissingUser
	}
	if leadID == uuid.Nil {
		return nil, ErrPendingReplyMissingLead
	}
	if channel != ChannelTelegram && channel != ChannelEmail {
		return nil, fmt.Errorf("%w: %q", ErrPendingReplyInvalidChannel, channel)
	}
	if !kind.IsValid() {
		return nil, fmt.Errorf("%w: %q", ErrPendingReplyInvalidKind, kind)
	}
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return nil, ErrPendingReplyEmptyBody
	}
	return &PendingReply{
		ID:        uuid.New(),
		UserID:    userID,
		LeadID:    leadID,
		Channel:   channel,
		Kind:      kind,
		Body:      trimmed,
		Status:    PendingReplyStatusPending,
		CreatedAt: time.Now().UTC(),
	}, nil
}
