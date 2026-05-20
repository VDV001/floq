package analytics

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// UseCase composes the analytics read-side. Today it just delegates
// to its port collaborators — kept as its own seam so future cross-
// context projections (e.g. cost-ratio composition over leads +
// audit_log + sequence stats joined together) don't have to rewire
// every handler.
type UseCase struct {
	seq  SequenceStatsReader
	cost CostRatiosReader
}

// NewUseCase wires the two read ports. Either may be nil if the
// surrounding wire-up only registers a subset of routes (tests do
// this — production wire-up always passes both).
func NewUseCase(seq SequenceStatsReader, cost CostRatiosReader) *UseCase {
	return &UseCase{seq: seq, cost: cost}
}

// GetSequenceStats forwards the call to the configured reader. Period
// validation happens at the handler boundary via ParsePeriod so this
// layer only sees the typed value.
func (uc *UseCase) GetSequenceStats(ctx context.Context, userID uuid.UUID, period Period) ([]SequenceStatsDTO, error) {
	return uc.seq.GetSequenceStats(ctx, userID, period)
}

// GetCostRatios forwards the call to the cost-ratios reader. The
// window [from, to) is resolved at the handler boundary from the
// period query param so the usecase stays time-source agnostic.
func (uc *UseCase) GetCostRatios(ctx context.Context, userID uuid.UUID, from, to time.Time) (*CostRatiosDTO, error) {
	return uc.cost.GetCostRatios(ctx, userID, from, to)
}
