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
	seq    SequenceStatsReader
	cost   CostRatiosReader
	hot    HotLeadsReader
	inbox  InboxFlowReader
	funnel FunnelReader
	// bucketStep is the configured score-bucket width for the
	// qualification-distribution funnel. The funnel matview is binned at
	// width 10; this folds it up to the operator's chosen step. Defaults
	// to 10, normalised via NormalizeBucketStep.
	bucketStep int
}

// Option configures optional read ports on the UseCase. Used for ports
// added after the original two so existing call sites stay unchanged.
type Option func(*UseCase)

// WithHotLeadsReader wires the View 4 hot-leads reader.
func WithHotLeadsReader(h HotLeadsReader) Option {
	return func(uc *UseCase) { uc.hot = h }
}

// WithInboxFlowReader wires the View 3 inbox-flow reader.
func WithInboxFlowReader(i InboxFlowReader) Option {
	return func(uc *UseCase) { uc.inbox = i }
}

// WithFunnelReader wires the matview-backed funnel reader (qualification
// distribution + sequence step conversion).
func WithFunnelReader(f FunnelReader) Option {
	return func(uc *UseCase) { uc.funnel = f }
}

// WithScoreBucketStep sets the qualification-distribution bucket width,
// normalised to a multiple of 10 in [10, 100].
func WithScoreBucketStep(step int) Option {
	return func(uc *UseCase) { uc.bucketStep = NormalizeBucketStep(step) }
}

// NewUseCase wires the read ports. seq and cost may be nil if the
// surrounding wire-up only registers a subset of routes (tests do
// this — production wire-up passes all). Optional readers are supplied
// via Option.
func NewUseCase(seq SequenceStatsReader, cost CostRatiosReader, opts ...Option) *UseCase {
	uc := &UseCase{seq: seq, cost: cost, bucketStep: 10}
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

// GetInboxFlow forwards to the inbox-flow reader. The [from, to) window
// is resolved at the handler boundary from the period query param.
func (uc *UseCase) GetInboxFlow(ctx context.Context, userID uuid.UUID, from, to time.Time) (*InboxFlowDTO, error) {
	return uc.inbox.GetInboxFlow(ctx, userID, from, to)
}

// GetCostRatios forwards the call to the cost-ratios reader. The
// window [from, to) is resolved at the handler boundary from the
// period query param so the usecase stays time-source agnostic.
func (uc *UseCase) GetCostRatios(ctx context.Context, userID uuid.UUID, from, to time.Time) (*CostRatiosDTO, error) {
	return uc.cost.GetCostRatios(ctx, userID, from, to)
}

// GetQualificationDistribution forwards to the funnel reader with the
// configured bucket step. The step is a server-side policy (not a request
// param), so it's injected at construction rather than parsed per request.
func (uc *UseCase) GetQualificationDistribution(ctx context.Context, userID uuid.UUID, period Period) (*QualificationFunnelDTO, error) {
	return uc.funnel.GetQualificationDistribution(ctx, userID, uc.bucketStep, period)
}

// GetSequenceConversion forwards to the funnel reader for the given period.
func (uc *UseCase) GetSequenceConversion(ctx context.Context, userID uuid.UUID, period Period) (*SequenceConversionDTO, error) {
	return uc.funnel.GetSequenceConversion(ctx, userID, period)
}
