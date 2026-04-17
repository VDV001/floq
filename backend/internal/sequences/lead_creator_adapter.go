package sequences

import (
	"context"
	"fmt"

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
	var emailPtr *string
	if prospect.Email != "" {
		emailPtr = &prospect.Email
	}
	lead, err := leadsdomain.NewLead(userID, leadsdomain.ChannelEmail, prospect.Name, prospect.Company, "Ответ на outbound секвенцию", nil, emailPtr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create lead entity: %w", err)
	}
	lead.SourceID = prospect.SourceID
	if err := a.repo.CreateLead(ctx, lead); err != nil {
		return uuid.Nil, err
	}
	return lead.ID, nil
}
