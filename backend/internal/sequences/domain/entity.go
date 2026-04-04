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

type SequenceStep struct {
	ID         uuid.UUID
	SequenceID uuid.UUID
	StepOrder  int
	DelayDays  int
	PromptHint string
	Channel    StepChannel
	CreatedAt  time.Time
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
