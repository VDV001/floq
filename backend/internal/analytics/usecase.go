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
	hot  HotLeadsReader
}

// Option configures optional read ports on the UseCase. Used for ports
// added after the original two so existing call sites stay unchanged.
type Option func(*UseCase)

// WithHotLeadsReader wires the View 4 hot-leads reader.
func WithHotLeadsReader(h HotLeadsReader) Option {
	return func(uc *UseCase) { uc.hot = h }
}

// NewUseCase wires the read ports. seq and cost may be nil if the
// surrounding wire-up only registers a subset of routes (tests do
// this — production wire-up passes all). Optional readers are supplied
// via Option.
func NewUseCase(seq SequenceStatsReader, cost CostRatiosReader, opts ...Option) *UseCase {
	uc := &UseCase{seq: seq, cost: cost}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

// GetHotLeads forwards to the hot-leads reader. The filter is validated
// at the handler boundary so this layer only sees typed values.
func (uc *UseCase) GetHotLeads(ctx context.Context, userID uuid.UUID, filter HotLeadsFilter) (*HotLeadsDTO, error) {
	return uc.hot.GetHotLeads(ctx, userID, filter)
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
