package inbox

import (
	"context"

	prospectsdomain "github.com/daniil/floq/internal/prospects/domain"
	"github.com/google/uuid"
)

type prospectRepoAdapter struct {
	repo prospectsdomain.Repository
}

func NewProspectRepoAdapter(repo prospectsdomain.Repository) ProspectRepository {
	return &prospectRepoAdapter{repo: repo}
}

func toMatch(p *prospectsdomain.Prospect) *ProspectMatch {
	if p == nil {
		return nil
	}
	return &ProspectMatch{
		ID:       p.ID,
		Name:     p.Name,
		Company:  p.Company,
		SourceID: p.SourceID,
		Status:   string(p.Status),
	}
}

func (a *prospectRepoAdapter) FindByEmail(ctx context.Context, userID uuid.UUID, email string) (*ProspectMatch, error) {
	p, err := a.repo.FindByEmail(ctx, userID, email)
	if err != nil {
		return nil, err
	}
	return toMatch(p), nil
}

func (a *prospectRepoAdapter) FindByTelegramUsername(ctx context.Context, userID uuid.UUID, username string) (*ProspectMatch, error) {
	p, err := a.repo.FindByTelegramUsername(ctx, userID, username)
	if err != nil {
		return nil, err
	}
	return toMatch(p), nil
}

func (a *prospectRepoAdapter) ConvertToLead(ctx context.Context, prospectID, leadID uuid.UUID) error {
	return a.repo.ConvertToLead(ctx, prospectID, leadID)
}
