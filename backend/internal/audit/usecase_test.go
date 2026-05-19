package audit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/daniil/floq/internal/audit"
	"github.com/daniil/floq/internal/audit/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAuditRepo struct {
	receivedFrom time.Time
	receivedTo   time.Time
	summary      *domain.CostSummary
	saveErr      error
	summaryErr   error
}

func (f *fakeAuditRepo) Save(_ context.Context, _ []*domain.Entry) error {
	return f.saveErr
}

func (f *fakeAuditRepo) CostSummary(_ context.Context, _ uuid.UUID, from, to time.Time) (*domain.CostSummary, error) {
	f.receivedFrom = from
	f.receivedTo = to
	if f.summaryErr != nil {
		return nil, f.summaryErr
	}
	if f.summary != nil {
		return f.summary, nil
	}
	return &domain.CostSummary{
		ByRequestType: []domain.RequestTypeBreakdown{},
		ByModel:       []domain.ModelBreakdown{},
		PeriodFrom:    from,
		PeriodTo:      to,
	}, nil
}

func TestUseCase_CostSummary_DefaultsToLast30Days(t *testing.T) {
	repo := &fakeAuditRepo{}
	uc := audit.NewUseCase(repo)
	_, err := uc.CostSummary(context.Background(), uuid.New(), time.Time{}, time.Time{})
	require.NoError(t, err)
	span := repo.receivedTo.Sub(repo.receivedFrom)
	// 30 days = 720h; allow a couple of seconds of slop for the clock
	// read inside the usecase.
	assert.InDelta(t, (30 * 24 * time.Hour).Seconds(), span.Seconds(), 2,
		"zero from + zero to must default to a 30-day window ending now")
}

func TestUseCase_CostSummary_RespectsExplicitRange(t *testing.T) {
	repo := &fakeAuditRepo{}
	uc := audit.NewUseCase(repo)
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	_, err := uc.CostSummary(context.Background(), uuid.New(), from, to)
	require.NoError(t, err)
	assert.True(t, repo.receivedFrom.Equal(from), "explicit from must pass through")
	assert.True(t, repo.receivedTo.Equal(to), "explicit to must pass through")
}

func TestUseCase_CostSummary_FromAfterToRejected(t *testing.T) {
	repo := &fakeAuditRepo{}
	uc := audit.NewUseCase(repo)
	from := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC) // before from
	_, err := uc.CostSummary(context.Background(), uuid.New(), from, to)
	require.Error(t, err)
	assert.ErrorIs(t, err, audit.ErrInvalidPeriod)
}

func TestUseCase_CostSummary_PropagatesRepoError(t *testing.T) {
	repo := &fakeAuditRepo{summaryErr: errors.New("db down")}
	uc := audit.NewUseCase(repo)
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	_, err := uc.CostSummary(context.Background(), uuid.New(), from, to)
	require.Error(t, err)
}
