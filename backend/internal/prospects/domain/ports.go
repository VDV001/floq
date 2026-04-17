package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository defines the persistence interface for prospects.
type Repository interface {
	ListProspects(ctx context.Context, userID uuid.UUID) ([]ProspectWithSource, error)
	GetProspect(ctx context.Context, id uuid.UUID) (*Prospect, error)
	FindByEmail(ctx context.Context, userID uuid.UUID, email string) (*Prospect, error)
	FindByTelegramUsername(ctx context.Context, userID uuid.UUID, username string) (*Prospect, error)
	// GetProspectForUser returns the prospect iff it belongs to userID;
	// returns (nil, nil) otherwise so callers translate to a tenant-safe
	// "not found" at their boundary.
	GetProspectForUser(ctx context.Context, userID, prospectID uuid.UUID) (*Prospect, error)
	CreateProspect(ctx context.Context, p *Prospect) error
	CreateProspectsBatch(ctx context.Context, prospects []Prospect) error
	DeleteProspect(ctx context.Context, id uuid.UUID) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status ProspectStatus) error
	ConvertToLead(ctx context.Context, prospectID, leadID uuid.UUID) error
	UpdateVerification(ctx context.Context, id uuid.UUID, verifyStatus VerifyStatus, verifyScore int, verifyDetails string, verifiedAt time.Time) error
}
