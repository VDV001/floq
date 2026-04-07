package sequences

import (
	"context"

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
		ID:               p.ID,
		UserID:           p.UserID,
		Name:             p.Name,
		Company:          p.Company,
		Title:            p.Title,
		Email:            p.Email,
		Phone:            p.Phone,
		WhatsApp:         p.WhatsApp,
		TelegramUsername: p.TelegramUsername,
		Context:          p.Context,
		Source:           p.Source,
		Status:           string(p.Status),
		VerifyStatus:     string(p.VerifyStatus),
		VerifiedAt:       p.VerifiedAt,
	}, nil
}

// UpdateStatus updates a prospect's status via the underlying repository.
func (a *ProspectReaderAdapter) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return a.repo.UpdateStatus(ctx, id, prospectsdomain.ProspectStatus(status))
}
