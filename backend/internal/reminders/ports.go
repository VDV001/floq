package reminders

import (
	"context"

	"github.com/google/uuid"

	"github.com/daniil/floq/internal/leads/domain"
)

// LeadRepository queries stale leads and persists reminders.
type LeadRepository interface {
	StaleLeadsWithoutReminder(ctx context.Context, staleDays int) ([]domain.Lead, error)
	CreateReminder(ctx context.Context, leadID uuid.UUID, message string) error
}

// FollowupGenerator produces AI-generated follow-up messages.
type FollowupGenerator interface {
	GenerateFollowup(ctx context.Context, contactName, company, daysAgo, lastMessage, ourLastReply string) (string, error)
}

// Notifier sends alert messages to the manager.
type Notifier interface {
	SendAlert(ctx context.Context, contactName, company, body string) error
}
