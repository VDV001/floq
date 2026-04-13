package sequences

import (
	"context"
	"time"

	leadsdomain "github.com/daniil/floq/internal/leads/domain"
	"github.com/daniil/floq/internal/sequences/domain"
	"github.com/google/uuid"
)

// LeadCreatorAdapter adapts leads.domain.Repository to sequences.domain.LeadCreator.
type LeadCreatorAdapter struct {
	repo leadsdomain.Repository
}

// NewLeadCreatorAdapter creates a new adapter.
func NewLeadCreatorAdapter(repo leadsdomain.Repository) *LeadCreatorAdapter {
	return &LeadCreatorAdapter{repo: repo}
}

// CreateLeadFromProspect creates a lead from prospect data and returns the new lead ID.
func (a *LeadCreatorAdapter) CreateLeadFromProspect(ctx context.Context, prospect *domain.ProspectView, userID uuid.UUID) (uuid.UUID, error) {
	leadID := uuid.New()
	now := time.Now().UTC()
	lead := &leadsdomain.Lead{
		ID:           leadID,
		UserID:       userID,
		Channel:      leadsdomain.ChannelEmail,
		ContactName:  prospect.Name,
		Company:      prospect.Company,
		FirstMessage: "Ответ на outbound секвенцию",
		Status:       leadsdomain.StatusNew,
		SourceID:     prospect.SourceID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := a.repo.CreateLead(ctx, lead); err != nil {
		return uuid.Nil, err
	}
	return leadID, nil
}
