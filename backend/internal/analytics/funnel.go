package analytics

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// QualBucketDTO is one folded score band in the qualification distribution.
// Public fields, no invariants — projection data, not a domain entity.
type QualBucketDTO struct {
	Lo    int
	Hi    int
	Label string
	Count int
}

// QualificationFunnelDTO is the score-distribution read model, folded from
// the matview's fixed width-10 bins up to the operator's configured step.
type QualificationFunnelDTO struct {
	Step    int
	Total   int
	Buckets []QualBucketDTO
}

// SequenceStepConversionDTO is one (sequence, step) funnel row: how many
// prospects received the step, replied, and advanced to the next step,
// plus the derived rates. Rates are reply/entered and advanced/entered.
type SequenceStepConversionDTO struct {
	SequenceID   uuid.UUID
	SequenceName string
	StepOrder    int
	Entered      int
	Replied      int
	Advanced     int
	ReplyRate    float64
	AdvanceRate  float64
}

// SequenceConversionDTO is the per-sequence step-conversion read model.
type SequenceConversionDTO struct {
	Steps []SequenceStepConversionDTO
}

// FunnelReader is the port the usecase depends on for the matview-backed
// funnel read-models. The pg implementation reads the materialized views
// refreshed by the background cron; tests stub it directly.
type FunnelReader interface {
	GetQualificationDistribution(ctx context.Context, userID uuid.UUID, step int, period Period) (*QualificationFunnelDTO, error)
	GetSequenceConversion(ctx context.Context, userID uuid.UUID, period Period) (*SequenceConversionDTO, error)
}

// Repository satisfies FunnelReader. Asserted here so a signature drift in
// the funnel feature breaks the build next to the methods it concerns.
var _ FunnelReader = (*Repository)(nil)

// NormalizeBucketStep clamps a configured score-bucket step to a multiple of
// 10 in [10, 100]; anything outside that falls back to 10. The matview bins
// are width 10, so only multiples of 10 fold cleanly.
func NormalizeBucketStep(step int) int {
	if step < 10 || step > 100 || step%10 != 0 {
		return 10
	}
	return step
}

// GetQualificationDistribution reads the per-tenant width-10 score bins from
// the matview and folds them up to step, emitting a complete histogram
// (zero-count buckets included) so the dashboard axis stays continuous. step
// is expected normalised (see NormalizeBucketStep) by the usecase.
func (r *Repository) GetQualificationDistribution(ctx context.Context, userID uuid.UUID, step int, period Period) (*QualificationFunnelDTO, error) {
	step = NormalizeBucketStep(step)
	_ = period // period filtering applied in a later step

	// Day-bucketed matview: sum the per-day counts (additive) into width-10
	// bins, then fold up to step below. A NULL cutoff means all-time.
	var cutoff any
	rows, err := r.pool.Query(ctx,
		`SELECT bucket_lo, SUM(cnt)::bigint
		 FROM mv_analytics_qualification_distribution
		 WHERE user_id = $1 AND ($2::date IS NULL OR day >= $2::date)
		 GROUP BY bucket_lo`,
		userID, cutoff,
	)
	if err != nil {
		return nil, fmt.Errorf("analytics qualification distribution: %w", err)
	}
	defer rows.Close()

	folded := map[int]int{}
	total := 0
	for rows.Next() {
		var lo10, cnt int
		if err := rows.Scan(&lo10, &cnt); err != nil {
			return nil, fmt.Errorf("analytics scan qualification distribution: %w", err)
		}
		folded[(lo10/step)*step] += cnt
		total += cnt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analytics qualification distribution iter: %w", err)
	}

	dto := &QualificationFunnelDTO{Step: step, Total: total}
	for lo := 0; lo < 100; lo += step {
		hi := lo + step - 1
		if lo+step >= 100 {
			// Top band is inclusive of the maximum score (100); the matview
			// folds score 100 into its highest width-10 bin.
			hi = 100
		}
		dto.Buckets = append(dto.Buckets, QualBucketDTO{
			Lo:    lo,
			Hi:    hi,
			Label: fmt.Sprintf("%d–%d", lo, hi),
			Count: folded[lo],
		})
	}
	return dto, nil
}

// GetSequenceConversion reads the per-(sequence, step) funnel from the
// matview, joining sequences for the current name (small, kept fresh rather
// than baked into the view) and deriving the reply/advance rates.
func (r *Repository) GetSequenceConversion(ctx context.Context, userID uuid.UUID, period Period) (*SequenceConversionDTO, error) {
	_ = period // period filtering applied in a later step

	// The matview is deduped to one row per (sequence, step, prospect) entered,
	// so a windowed COUNT/COUNT FILTER is exact (no distinct-additivity hazard).
	// A NULL cutoff means all-time.
	var cutoff any
	rows, err := r.pool.Query(ctx, `
		SELECT c.sequence_id, s.name, c.step_order,
		       COUNT(*)::bigint                              AS entered,
		       (COUNT(*) FILTER (WHERE c.replied))::bigint   AS replied,
		       (COUNT(*) FILTER (WHERE c.advanced))::bigint  AS advanced
		FROM mv_analytics_sequence_step_conversion c
		JOIN sequences s ON s.id = c.sequence_id
		WHERE c.user_id = $1 AND ($2::timestamptz IS NULL OR c.entered_at >= $2)
		GROUP BY c.sequence_id, s.name, c.step_order
		ORDER BY s.name, c.step_order`,
		userID, cutoff,
	)
	if err != nil {
		return nil, fmt.Errorf("analytics sequence conversion: %w", err)
	}
	defer rows.Close()

	dto := &SequenceConversionDTO{Steps: []SequenceStepConversionDTO{}}
	for rows.Next() {
		var row SequenceStepConversionDTO
		if err := rows.Scan(&row.SequenceID, &row.SequenceName, &row.StepOrder,
			&row.Entered, &row.Replied, &row.Advanced); err != nil {
			return nil, fmt.Errorf("analytics scan sequence conversion: %w", err)
		}
		row.ReplyRate = safeRate(row.Replied, row.Entered)
		row.AdvanceRate = safeRate(row.Advanced, row.Entered)
		dto.Steps = append(dto.Steps, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analytics sequence conversion iter: %w", err)
	}
	return dto, nil
}

// funnelMatviews are the materialized views the refresh cron rebuilds.
var funnelMatviews = []string{
	"mv_analytics_qualification_distribution",
	"mv_analytics_sequence_step_conversion",
}

// RefreshMatviews rebuilds the funnel materialized views CONCURRENTLY so
// readers keep serving the previous snapshot during the refresh. Requires
// the UNIQUE index each view carries (migration 042) and that the view was
// already populated once (created WITH DATA). Returns the first error.
func (r *Repository) RefreshMatviews(ctx context.Context) error {
	for _, mv := range funnelMatviews {
		if _, err := r.pool.Exec(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY "+mv); err != nil {
			return fmt.Errorf("refresh %s: %w", mv, err)
		}
	}
	return nil
}

// safeRate divides num by denom, returning 0 for a non-positive denominator
// rather than emitting NaN/Inf at the wire boundary.
func safeRate(num, denom int) float64 {
	if denom <= 0 {
		return 0
	}
	return float64(num) / float64(denom)
}
