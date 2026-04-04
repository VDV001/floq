package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// DetectCallAgreement checks if the message indicates the person agrees to a call/meeting.
func DetectCallAgreement(text string) bool {
	lower := strings.ToLower(text)
	markers := []string{
		"давайте созвон", "давай созвон", "готов созвон", "согласен на созвон",
		"можно созвон", "давайте звонок", "давай звонок", "готов к звонку",
		"давайте встреч", "давай встреч", "согласен на встреч", "готов встретить",
		"можем созвон", "можем встретить", "давайте обсудим", "готов обсудить",
		"да, давайте", "да давайте", "конечно, давайте", "с удовольствием",
		"когда удобно", "выберу время", "забронир", "запишусь",
		"да, можно", "да можно", "ок, давай", "ок давай",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// --- LeadStatus value object ---

type LeadStatus string

const (
	StatusNew       LeadStatus = "new"
	StatusQualified LeadStatus = "qualified"
	StatusClosed    LeadStatus = "closed"
	StatusWon       LeadStatus = "won"
)

var allowedTransitions = map[LeadStatus][]LeadStatus{
	StatusNew:       {StatusQualified, StatusClosed},
	StatusQualified: {StatusClosed, StatusWon},
	StatusWon:       {StatusClosed},
}

func (s LeadStatus) IsValid() bool {
	switch s {
	case StatusNew, StatusQualified, StatusClosed, StatusWon:
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
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Message struct {
	ID        uuid.UUID
	LeadID    uuid.UUID
	Direction MessageDirection
	Body      string
	SentAt    time.Time
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
