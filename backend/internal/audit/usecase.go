package audit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/daniil/floq/internal/audit/domain"
	"github.com/google/uuid"
)

// ErrInvalidPeriod is returned when the cost-summary request asks for
// a zero-or-negative span (to <= from). The handler maps this to 400.
var ErrInvalidPeriod = errors.New("audit: invalid period (to must be after from)")

// defaultLookbackDays is the period that GET /api/audit/cost-summary
// uses when the caller omits both from and to query parameters.
// Thirty days mirrors typical monthly billing-review cadence.
const defaultLookbackDays = 30

// UseCase wraps the audit repository with input validation + sensible
// defaults so the handler is a pure parse-and-map layer.
type UseCase struct {
	repo domain.AuditRepository
	now  func() time.Time
}

func NewUseCase(repo domain.AuditRepository) *UseCase {
	return &UseCase{repo: repo, now: time.Now}
}

// CostSummary returns the per-user spend ledger over the requested
// range. Zero-valued from or to (the handler signals "missing query
// param" with the zero value) default to "last 30 days ending now".
func (uc *UseCase) CostSummary(ctx context.Context, userID uuid.UUID, from, to time.Time) (*domain.CostSummary, error) {
	if to.IsZero() {
		to = uc.now().UTC()
	}
	if from.IsZero() {
		from = to.AddDate(0, 0, -defaultLookbackDays)
	}
	if !to.After(from) {
		return nil, fmt.Errorf("%w: from=%s to=%s", ErrInvalidPeriod, from.Format(time.RFC3339), to.Format(time.RFC3339))
	}
	return uc.repo.CostSummary(ctx, userID, from, to)
}
