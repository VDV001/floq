package main

import (
	"context"

	"github.com/daniil/floq/internal/inbox"
	leadsdomain "github.com/daniil/floq/internal/leads/domain"
	"github.com/daniil/floq/internal/prospects"
	prospectsdomain "github.com/daniil/floq/internal/prospects/domain"
	"github.com/google/uuid"
)

// --- LeadChecker adapter (prospects → leads boundary) ---

type leadCheckerAdapter struct {
	repo leadsdomain.Repository
}

func newLeadCheckerAdapter(repo leadsdomain.Repository) prospects.LeadChecker {
	return &leadCheckerAdapter{repo: repo}
}

func (a *leadCheckerAdapter) LeadExistsByEmail(ctx context.Context, userID uuid.UUID, email string) (bool, error) {
	lead, err := a.repo.GetLeadByEmailAddress(ctx, userID, email)
	if err != nil {
		return false, err
	}
	return lead != nil, nil
}

// --- ProspectRepo adapter (inbox → prospects boundary) ---

type prospectRepoAdapter struct {
	repo prospectsdomain.Repository
}

func newProspectRepoAdapter(repo prospectsdomain.Repository) inbox.ProspectRepository {
	return &prospectRepoAdapter{repo: repo}
}

func toProspectMatch(p *prospectsdomain.Prospect) *inbox.ProspectMatch {
	if p == nil {
		return nil
	}
	return &inbox.ProspectMatch{
		ID:       p.ID,
		Name:     p.Name,
		Company:  p.Company,
		SourceID: p.SourceID,
		Status:   string(p.Status),
	}
}

func (a *prospectRepoAdapter) FindByEmail(ctx context.Context, userID uuid.UUID, email string) (*inbox.ProspectMatch, error) {
	p, err := a.repo.FindByEmail(ctx, userID, email)
	if err != nil {
		return nil, err
	}
	return toProspectMatch(p), nil
}

func (a *prospectRepoAdapter) FindByTelegramUsername(ctx context.Context, userID uuid.UUID, username string) (*inbox.ProspectMatch, error) {
	p, err := a.repo.FindByTelegramUsername(ctx, userID, username)
	if err != nil {
		return nil, err
	}
	return toProspectMatch(p), nil
}

func (a *prospectRepoAdapter) ConvertToLead(ctx context.Context, prospectID, leadID uuid.UUID) error {
	return a.repo.ConvertToLead(ctx, prospectID, leadID)
}
