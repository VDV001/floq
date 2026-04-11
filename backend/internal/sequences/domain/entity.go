package domain

import (
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
func NewSequence(userID uuid.UUID, name string) *Sequence {
	return &Sequence{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      name,
		IsActive:  false,
		CreatedAt: time.Now().UTC(),
	}
}

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
	CreatedAt   time.Time
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
