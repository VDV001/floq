package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/daniil/floq/internal/audit/domain"
)

// RetentionUseCase turns a retention window (in days) into the concrete
// cut-off timestamp and drives the repository roll-up-and-purge. It owns
// the clock so the handler/cron stay free of time.Now — the retention
// boundary is a policy decision, not a transport concern.
type RetentionUseCase struct {
	repo          domain.RetentionRepository
	retentionDays int
	now           func() time.Time
}

// NewRetentionUseCase builds the use case. retentionDays is how long the
// per-call audit_log rows are kept before being aggregated into the
// daily rollup and deleted.
func NewRetentionUseCase(repo domain.RetentionRepository, retentionDays int) *RetentionUseCase {
	return &RetentionUseCase{repo: repo, retentionDays: retentionDays, now: time.Now}
}

// Purge aggregates and deletes every audit_log row older than
// now-retentionDays, returning how many rows were purged.
func (uc *RetentionUseCase) Purge(ctx context.Context) (int, error) {
	threshold := uc.now().AddDate(0, 0, -uc.retentionDays)
	purged, err := uc.repo.AggregateAndPurgeOlderThan(ctx, threshold)
	if err != nil {
		return 0, fmt.Errorf("audit retention purge: %w", err)
	}
	return purged, nil
}
