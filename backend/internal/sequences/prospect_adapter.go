package sequences

import (
	"context"
	"fmt"

	prospectsdomain "github.com/daniil/floq/internal/prospects/domain"
	"github.com/daniil/floq/internal/sequences/domain"
	"github.com/google/uuid"
)

// ProspectReaderAdapter adapts prospects.domain.Repository to sequences.domain.ProspectReader.
type ProspectReaderAdapter struct {
	repo prospectsdomain.Repository
}

// NewProspectReaderAdapter creates a new adapter.
func NewProspectReaderAdapter(repo prospectsdomain.Repository) *ProspectReaderAdapter {
	return &ProspectReaderAdapter{repo: repo}
}

// GetProspect retrieves a prospect and maps it to the sequences domain's ProspectView.
func (a *ProspectReaderAdapter) GetProspect(ctx context.Context, id uuid.UUID) (*domain.ProspectView, error) {
	p, err := a.repo.GetProspect(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, nil
	}
	return &domain.ProspectView{
		ID:                    p.ID,
		UserID:                p.UserID,
		Name:                  p.Name,
		Company:               p.Company,
		Title:                 p.Title,
		Email:                 p.Email,
		Phone:                 p.Phone,
		WhatsApp:              p.WhatsApp,
		TelegramUsername:      p.TelegramUsername,
		Context:               p.Context,
		Source:                p.Source,
		SourceID:              p.SourceID,
		Status:                string(p.Status),
		VerifyStatus:          string(p.VerifyStatus),
		VerifiedAt:            p.VerifiedAt,
		IsEligibleForSequence: p.CanLaunchSequence(), // delegate to prospects domain rule
	}, nil
}

// transitionProspect loads a prospect, asks the domain entity to transition
// to the target status (state machine enforces legality), and persists.
// Shared helper for the named transition methods below.
func (a *ProspectReaderAdapter) transitionProspect(ctx context.Context, id uuid.UUID, target prospectsdomain.ProspectStatus) error {
	p, err := a.repo.GetProspect(ctx, id)
	if err != nil {
		return fmt.Errorf("transition prospect: load: %w", err)
	}
	if p == nil {
		return fmt.Errorf("transition prospect: not found")
	}
	if err := p.TransitionTo(target); err != nil {
		return fmt.Errorf("transition prospect: %w", err)
	}
	return a.repo.UpdateStatus(ctx, id, p.Status)
}

// MarkInSequence implements the domain.ProspectReader contract by translating
// the sequences-side intent into the prospects-domain enum value. sequences
// never sees the string "in_sequence" directly.
func (a *ProspectReaderAdapter) MarkInSequence(ctx context.Context, id uuid.UUID) error {
	return a.transitionProspect(ctx, id, prospectsdomain.ProspectStatusInSequence)
}

// MarkConverted moves the prospect into the terminal Converted state.
func (a *ProspectReaderAdapter) MarkConverted(ctx context.Context, id uuid.UUID) error {
	return a.transitionProspect(ctx, id, prospectsdomain.ProspectStatusConverted)
}
