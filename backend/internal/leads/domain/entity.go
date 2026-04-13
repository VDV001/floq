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
	SourceName     string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewLead creates a new Lead with generated ID, status=new, and timestamps.
func NewLead(userID uuid.UUID, channel Channel, contactName, company, firstMessage string, telegramChatID *int64, emailAddress *string) *Lead {
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
	}
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

type Draft struct {
	ID        uuid.UUID
	LeadID    uuid.UUID
	Body      string
	CreatedAt time.Time
}

type Reminder struct {
	ID        uuid.UUID
	LeadID    uuid.UUID
	Message   string
	CreatedAt time.Time
	Dismissed bool
}
