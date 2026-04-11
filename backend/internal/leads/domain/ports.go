package domain

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines persistence operations for the leads domain.
type Repository interface {
	ListLeads(ctx context.Context, userID uuid.UUID) ([]Lead, error)
	GetLead(ctx context.Context, id uuid.UUID) (*Lead, error)
	CreateLead(ctx context.Context, lead *Lead) error
	UpdateFirstMessage(ctx context.Context, id uuid.UUID, message string) error
	UpdateLeadStatus(ctx context.Context, id uuid.UUID, status LeadStatus) error
	GetLeadByTelegramChatID(ctx context.Context, userID uuid.UUID, chatID int64) (*Lead, error)
	GetLeadByEmailAddress(ctx context.Context, userID uuid.UUID, email string) (*Lead, error)
	StaleLeadsWithoutReminder(ctx context.Context, staleDays int) ([]Lead, error)

	ListMessages(ctx context.Context, leadID uuid.UUID) ([]Message, error)
	CreateMessage(ctx context.Context, msg *Message) error

	GetQualification(ctx context.Context, leadID uuid.UUID) (*Qualification, error)
	UpsertQualification(ctx context.Context, q *Qualification) error

	GetLatestDraft(ctx context.Context, leadID uuid.UUID) (*Draft, error)
	CreateDraft(ctx context.Context, d *Draft) error

	CreateReminder(ctx context.Context, leadID uuid.UUID, message string) error

	CountMonthLeads(ctx context.Context, userID uuid.UUID) (int, error)
	CountTotalLeads(ctx context.Context, userID uuid.UUID) (int, error)
}

// MessageSender sends messages to leads via their channel.
type MessageSender interface {
	SendMessage(ctx context.Context, lead *Lead, body string) error
}

// AIService provides AI-powered operations for leads.
type AIService interface {
	Qualify(ctx context.Context, contactName string, channel Channel, firstMessage string) (*Qualification, error)
	DraftReply(ctx context.Context, contactName string, firstMessage string) (string, error)
	GenerateFollowup(ctx context.Context, contactName string, company string, days int) (string, error)
}

// Notifier sends alerts and notifications.
type Notifier interface {
	SendAlert(ctx context.Context, leadName string, company string, message string) error
}
