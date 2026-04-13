package inbox

import (
	"context"

	"github.com/google/uuid"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/leads/domain"
	settingsdomain "github.com/daniil/floq/internal/settings/domain"
)

// LeadRepository is the interface inbox needs from leads.
type LeadRepository interface {
	GetLeadByTelegramChatID(ctx context.Context, userID uuid.UUID, chatID int64) (*domain.Lead, error)
	GetLeadByEmailAddress(ctx context.Context, userID uuid.UUID, email string) (*domain.Lead, error)
	CreateLead(ctx context.Context, lead *domain.Lead) error
	UpdateFirstMessage(ctx context.Context, id uuid.UUID, message string) error
	CreateMessage(ctx context.Context, msg *domain.Message) error
	UpsertQualification(ctx context.Context, q *domain.Qualification) error
	UpdateLeadStatus(ctx context.Context, id uuid.UUID, status domain.LeadStatus) error
}

// ProspectMatch is a port-level read model for prospect data used by inbox.
type ProspectMatch struct {
	ID       uuid.UUID
	Name     string
	Company  string
	SourceID *uuid.UUID
	Status   string
}

const ProspectStatusConverted = "converted"

// ProspectRepository is the interface inbox needs from prospects.
type ProspectRepository interface {
	FindByEmail(ctx context.Context, userID uuid.UUID, email string) (*ProspectMatch, error)
	FindByTelegramUsername(ctx context.Context, userID uuid.UUID, username string) (*ProspectMatch, error)
	ConvertToLead(ctx context.Context, prospectID, leadID uuid.UUID) error
}

// SequenceRepository is the interface inbox needs from sequences.
type SequenceRepository interface {
	MarkRepliedByProspect(ctx context.Context, prospectID uuid.UUID) error
}

// AIQualifier qualifies leads using AI.
type AIQualifier interface {
	Qualify(ctx context.Context, contactName, channel, firstMessage string) (*ai.QualificationResult, error)
	ProviderName() string
}

// ConfigStore reads user configuration.
type ConfigStore interface {
	GetConfig(ctx context.Context, userID uuid.UUID) (*settingsdomain.UserConfig, error)
}
