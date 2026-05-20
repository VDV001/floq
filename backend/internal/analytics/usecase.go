package analytics

import (
	"context"

	"github.com/google/uuid"
)

// UseCase composes the analytics read-side. Today it just delegates
// to the reader port — kept as its own seam so future cross-context
// projections (e.g. cost-ratio composition over leads + audit_log)
// don't have to rewire every handler.
type UseCase struct {
	reader SequenceStatsReader
}

func NewUseCase(reader SequenceStatsReader) *UseCase {
	return &UseCase{reader: reader}
}

// GetSequenceStats forwards the call to the configured reader. Period
// validation happens at the handler boundary via ParsePeriod so this
// layer only sees the typed value.
func (uc *UseCase) GetSequenceStats(ctx context.Context, userID uuid.UUID, period Period) ([]SequenceStatsDTO, error) {
	return uc.reader.GetSequenceStats(ctx, userID, period)
}
