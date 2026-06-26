package leads

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchiveLead_HappyPath(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, Status: domain.StatusQualified}
	uc := NewUseCase(repo, &mockAI{}, nil)

	err := uc.ArchiveLead(context.Background(), leadID)
	require.NoError(t, err)
	require.NotNil(t, repo.leads[leadID].ArchivedAt, "lead should be archived")
	assert.Equal(t, domain.StatusQualified, repo.leads[leadID].Status, "archive must not change status")
}

func TestArchiveLead_NotFound(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, &mockAI{}, nil)

	err := uc.ArchiveLead(context.Background(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrLeadNotFound)
}

func TestArchiveLead_AlreadyArchived(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	lead := &domain.Lead{ID: leadID, Status: domain.StatusNew}
	require.NoError(t, lead.Archive())
	repo.leads[leadID] = lead
	uc := NewUseCase(repo, &mockAI{}, nil)

	err := uc.ArchiveLead(context.Background(), leadID)
	assert.ErrorIs(t, err, domain.ErrAlreadyArchived)
}

func TestUnarchiveLead_HappyPath(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	lead := &domain.Lead{ID: leadID, Status: domain.StatusNew}
	require.NoError(t, lead.Archive())
	repo.leads[leadID] = lead
	uc := NewUseCase(repo, &mockAI{}, nil)

	err := uc.UnarchiveLead(context.Background(), leadID)
	require.NoError(t, err)
	assert.Nil(t, repo.leads[leadID].ArchivedAt, "lead should be unarchived")
}

func TestUnarchiveLead_NotFound(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, &mockAI{}, nil)

	err := uc.UnarchiveLead(context.Background(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrLeadNotFound)
}

func TestUnarchiveLead_NotArchived(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, Status: domain.StatusNew}
	uc := NewUseCase(repo, &mockAI{}, nil)

	err := uc.UnarchiveLead(context.Background(), leadID)
	assert.ErrorIs(t, err, domain.ErrNotArchived)
}
