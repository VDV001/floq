package prospects

import (
	"context"

	leadsdomain "github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
)

type LeadCheckerAdapter struct {
	repo leadsdomain.Repository
}

func NewLeadCheckerAdapter(repo leadsdomain.Repository) *LeadCheckerAdapter {
	return &LeadCheckerAdapter{repo: repo}
}

func (a *LeadCheckerAdapter) LeadExistsByEmail(ctx context.Context, userID uuid.UUID, email string) (bool, error) {
	lead, err := a.repo.GetLeadByEmailAddress(ctx, userID, email)
	if err != nil {
		return false, err
	}
	return lead != nil, nil
}
