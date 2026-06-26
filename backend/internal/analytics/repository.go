package analytics

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository reads aggregated analytics rows from PG. Pure read-side:
// no mutations, no domain invariants — projections over the existing
// sequences / outbound_messages / prospects tables for the dashboard.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Compile-time check: Repository satisfies the ports consumed by the
// usecases. Adding the assertions here means a signature drift breaks
// the build, not a runtime cast at wire-up time.
var (
	_ SequenceStatsReader = (*Repository)(nil)
	_ CostRatiosReader    = (*Repository)(nil)
	_ HotLeadsReader      = (*Repository)(nil)
	_ InboxFlowReader     = (*Repository)(nil)
)

// GetInboxFlow composes the View 3 inbound-funnel read model over the
// [from, to) window: lead volume sliced by channel and status, the
// qualification-score histogram + mean, and the HITL approval stats
// (approve/reject/pending counts + time-to-decide percentiles). Three
// sequential queries — clarity over throughput, the dashboard is not a
// hot path (mirrors GetCostRatios).
//
// Every section is tenant-scoped through its own user_id column. The
// histogram bands come from the shared scoreBuckets table so the SQL
// FILTERs and the DTO labels stay in lockstep.
func (r *Repository) GetInboxFlow(ctx context.Context, userID uuid.UUID, from, to time.Time) (*InboxFlowDTO, error) {
	dto := &InboxFlowDTO{
		PeriodFrom: from,
		PeriodTo:   to,
		Leads:      LeadsBreakdownDTO{ByChannel: map[string]int{}, ByStatus: map[string]int{}},
	}

	if err := r.loadLeadsBreakdown(ctx, userID, from, to, &dto.Leads); err != nil {
		return nil, err
	}
	if err := r.loadQualificationDistribution(ctx, userID, from, to, &dto.Qualifications); err != nil {
		return nil, err
	}
	if err := r.loadPendingRepliesStats(ctx, userID, from, to, &dto.PendingReplies); err != nil {
		return nil, err
	}
	return dto, nil
}

// loadLeadsBreakdown fills total/by-channel/by-status from a single
// GROUP BY channel, status. Counts are folded into the maps in Go so the
// query stays one round-trip regardless of how many enum members are
// present.
func (r *Repository) loadLeadsBreakdown(ctx context.Context, userID uuid.UUID, from, to time.Time, out *LeadsBreakdownDTO) error {
	rows, err := r.pool.Query(ctx, `
		SELECT channel::text, status::text, COUNT(*)
		FROM leads
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
		  AND archived_at IS NULL
		GROUP BY channel, status`,
		userID, from, to,
	)
	if err != nil {
		return fmt.Errorf("analytics inbox leads breakdown: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			channel, status string
			count           int
		)
		if err := rows.Scan(&channel, &status, &count); err != nil {
			return fmt.Errorf("analytics inbox scan leads breakdown: %w", err)
		}
		out.Total += count
		out.ByChannel[channel] += count
		out.ByStatus[status] += count
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("analytics inbox leads breakdown iter: %w", err)
	}
	return nil
}

// loadQualificationDistribution fills the score histogram and mean. The
// per-band COUNT(*) FILTER columns are built from scoreBuckets so the
// SQL and the DTO labels can't drift; the bounds are compile-time int
// constants so the fmt-built SQL carries no injection surface. Period
// scope is on the lead's created_at (same cohort as the leads slice).
func (r *Repository) loadQualificationDistribution(ctx context.Context, userID uuid.UUID, from, to time.Time, out *QualificationDistributionDTO) error {
	var sb strings.Builder
	sb.WriteString("SELECT COALESCE(AVG(q.score), 0)")
	for i, b := range scoreBuckets {
		fmt.Fprintf(&sb, ", COUNT(*) FILTER (WHERE q.score BETWEEN %d AND %d) AS b%d", b.Lo, b.Hi, i)
	}
	sb.WriteString(`
		FROM qualifications q
		JOIN leads l ON l.id = q.lead_id
		WHERE l.user_id = $1 AND l.created_at >= $2 AND l.created_at < $3
		  AND l.archived_at IS NULL`)

	counts := make([]int, len(scoreBuckets))
	dests := make([]any, 0, len(scoreBuckets)+1)
	dests = append(dests, &out.AvgScore)
	for i := range counts {
		dests = append(dests, &counts[i])
	}
	if err := r.pool.QueryRow(ctx, sb.String(), userID, from, to).Scan(dests...); err != nil {
		return fmt.Errorf("analytics inbox qualification histogram: %w", err)
	}

	out.ScoreHistogram = make([]ScoreBucketDTO, len(scoreBuckets))
	for i, b := range scoreBuckets {
		out.ScoreHistogram[i] = ScoreBucketDTO{Range: b.Label, Count: counts[i]}
	}
	return nil
}

// loadPendingRepliesStats fills the HITL approval slice. approved counts
// the approved+sent terminal states (both operator approvals); the
// percentiles are computed in SQL over decided rows (decided_at NOT
// NULL) via percentile_cont, COALESCEd to 0 so an empty queue scans a
// number rather than NULL. Seconds are rounded to whole integers at the
// boundary.
func (r *Repository) loadPendingRepliesStats(ctx context.Context, userID uuid.UUID, from, to time.Time, out *PendingRepliesStatsDTO) error {
	var p50, p95 float64
	err := r.pool.QueryRow(ctx, `
		SELECT
		    COUNT(*) FILTER (WHERE status IN ('approved', 'sent')) AS approved,
		    COUNT(*) FILTER (WHERE status = 'rejected')           AS rejected,
		    COUNT(*) FILTER (WHERE status = 'pending')            AS pending,
		    COALESCE(percentile_cont(0.5) WITHIN GROUP (
		        ORDER BY EXTRACT(EPOCH FROM (decided_at - created_at))
		    ) FILTER (WHERE decided_at IS NOT NULL), 0) AS p50,
		    COALESCE(percentile_cont(0.95) WITHIN GROUP (
		        ORDER BY EXTRACT(EPOCH FROM (decided_at - created_at))
		    ) FILTER (WHERE decided_at IS NOT NULL), 0) AS p95
		FROM pending_replies
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3`,
		userID, from, to,
	).Scan(&out.Approved, &out.Rejected, &out.CurrentlyPending, &p50, &p95)
	if err != nil {
		return fmt.Errorf("analytics inbox pending replies: %w", err)
	}
	out.P50TimeToDecideSeconds = int(math.Round(p50))
	out.P95TimeToDecideSeconds = int(math.Round(p95))
	return nil
}

// GetSequenceStats returns one row per sequence with activity in the
// requested period. Sequences with zero outbound rows in the window
// are filtered out — the dashboard shows what's running, not the
// empty rolls.
//
// Tenant scope is enforced through sequences.user_id; the LEFT JOIN
// to prospects exists only so we can read p.status for the converted
// count, which is DISTINCT prospect_id so multiple outbounds to one
// converted prospect count as one conversion.
func (r *Repository) GetSequenceStats(ctx context.Context, userID uuid.UUID, period Period) ([]SequenceStatsDTO, error) {
	cutoff, hasCutoff := periodCutoff(period, time.Now().UTC())

	rows, err := r.pool.Query(ctx, `
		SELECT
		    s.id,
		    s.name,
		    COUNT(*) FILTER (WHERE om.status IN ('sent', 'bounced')) AS sent,
		    COUNT(*) FILTER (WHERE om.status = 'sent') AS delivered,
		    COUNT(*) FILTER (WHERE om.opened_at IS NOT NULL) AS opened,
		    COUNT(*) FILTER (WHERE om.replied_at IS NOT NULL) AS replied,
		    COUNT(DISTINCT om.prospect_id) FILTER (WHERE p.status = 'converted') AS converted
		FROM sequences s
		JOIN outbound_messages om ON om.sequence_id = s.id
		LEFT JOIN prospects p ON p.id = om.prospect_id
		WHERE s.user_id = $1
		  AND ($2::timestamptz IS NULL OR om.created_at >= $2)
		GROUP BY s.id, s.name
		ORDER BY s.name`,
		userID, nullableCutoff(cutoff, hasCutoff),
	)
	if err != nil {
		return nil, fmt.Errorf("analytics get sequence stats: %w", err)
	}
	defer rows.Close()

	out := []SequenceStatsDTO{}
	for rows.Next() {
		var row SequenceStatsDTO
		if err := rows.Scan(&row.ID, &row.Name, &row.Sent, &row.Delivered, &row.Opened, &row.Replied, &row.Converted); err != nil {
			return nil, fmt.Errorf("analytics scan sequence stats: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analytics rows iter: %w", err)
	}
	return out, nil
}

// periodCutoff maps the Period enum to the SQL cutoff timestamp. The
// boolean signals "no cutoff" (PeriodAll) so callers don't conflate
// the zero-time with a real cutoff at the Unix epoch.
func periodCutoff(p Period, now time.Time) (time.Time, bool) {
	switch p {
	case PeriodWeek:
		return now.Add(-7 * 24 * time.Hour), true
	case PeriodMonth:
		return now.Add(-30 * 24 * time.Hour), true
	default:
		return time.Time{}, false
	}
}

// nullableCutoff returns an interface that pgx serialises as SQL NULL
// when hasCutoff is false. This lets the query keep a single shape
// regardless of whether a window filter is in effect.
func nullableCutoff(t time.Time, hasCutoff bool) any {
	if !hasCutoff {
		return nil
	}
	return t
}

// GetCostRatios composes the View 2 cost dashboard read model: the
// total AI spend over [from, to) plus the four denominator counts
// (leads / qualified leads / converted prospects / sent outbounds)
// the ratios depend on. Five sequential queries — clarity over
// throughput; the dashboard is not a hot path.
//
// Ratios are computed inside the repo so the wire surface stays
// integer-pure (USD micro-units divided by counts). A zero
// denominator yields a zero ratio rather than panicking or emitting
// IEEE-754 Inf.
func (r *Repository) GetCostRatios(ctx context.Context, userID uuid.UUID, from, to time.Time) (*CostRatiosDTO, error) {
	dto := &CostRatiosDTO{
		PeriodFrom: from,
		PeriodTo:   to,
	}

	if err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(cost_usd_micro), 0), COUNT(*)
		 FROM audit_log
		 WHERE user_id = $1 AND created_at >= $2 AND created_at < $3`,
		userID, from, to,
	).Scan(&dto.TotalCostUSDMicro, &dto.TotalCalls); err != nil {
		return nil, fmt.Errorf("analytics cost-ratios audit: %w", err)
	}

	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM leads
		 WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
		   AND archived_at IS NULL`,
		userID, from, to,
	).Scan(&dto.LeadsCount); err != nil {
		return nil, fmt.Errorf("analytics cost-ratios leads: %w", err)
	}

	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM (
			SELECT l.id FROM leads l
			JOIN qualifications q ON q.lead_id = l.id
			WHERE l.user_id = $1 AND l.created_at >= $2 AND l.created_at < $3
			  AND l.archived_at IS NULL
			GROUP BY l.id
			HAVING MAX(q.score) >= $4
		) sub`,
		userID, from, to, QualifiedScoreThreshold,
	).Scan(&dto.QualifiedLeadsCount); err != nil {
		return nil, fmt.Errorf("analytics cost-ratios qualified: %w", err)
	}

	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM prospects
		 WHERE user_id = $1 AND status = 'converted'
		   AND updated_at >= $2 AND updated_at < $3`,
		userID, from, to,
	).Scan(&dto.ConvertedCount); err != nil {
		return nil, fmt.Errorf("analytics cost-ratios converted: %w", err)
	}

	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM outbound_messages om
		 JOIN prospects p ON p.id = om.prospect_id
		 WHERE p.user_id = $1 AND om.status = 'sent'
		   AND om.sent_at >= $2 AND om.sent_at < $3`,
		userID, from, to,
	).Scan(&dto.DraftsSentCount); err != nil {
		return nil, fmt.Errorf("analytics cost-ratios drafts: %w", err)
	}

	dto.CostPerLeadUSDMicro = safeRatioInt(dto.TotalCostUSDMicro, dto.LeadsCount)
	dto.CostPerQualifiedUSDMicro = safeRatioInt(dto.TotalCostUSDMicro, dto.QualifiedLeadsCount)
	dto.CostPerConvertedUSDMicro = safeRatioInt(dto.TotalCostUSDMicro, dto.ConvertedCount)
	dto.CostPerDraftSentUSDMicro = safeRatioInt(dto.TotalCostUSDMicro, dto.DraftsSentCount)
	return dto, nil
}

// GetHotLeads returns the ranked lead list for View 4: leads LEFT JOIN
// their (1:1, lead_id is UNIQUE) qualification, scored highest-first.
// A single query with COUNT(*) OVER() yields the page and the pre-LIMIT
// total in one round-trip. Unqualified leads keep a NULL score and sort
// last (NULLS LAST). status=any excludes the terminal 'closed' state;
// an explicit status returns exactly that state. Archived leads
// (archived_at IS NOT NULL) are excluded regardless of status.
//
// Tenant scope via leads.user_id. Sort is (score DESC NULLS LAST,
// updated_at DESC) so the freshest hottest leads surface first.
func (r *Repository) GetHotLeads(ctx context.Context, userID uuid.UUID, filter HotLeadsFilter) (*HotLeadsDTO, error) {
	cutoff, hasCutoff := periodCutoff(filter.Period, time.Now().UTC())

	rows, err := r.pool.Query(ctx, `
		SELECT l.id, l.contact_name, l.channel::text, l.status::text, l.updated_at,
		       q.score, q.score_reason, q.generated_at,
		       COUNT(*) OVER() AS total_matching
		FROM leads l
		LEFT JOIN qualifications q ON q.lead_id = l.id
		WHERE l.user_id = $1
		  AND l.archived_at IS NULL
		  AND ($2::timestamptz IS NULL OR l.created_at >= $2)
		  AND (
		        ($3 = 'any' AND l.status::text <> 'closed')
		     OR ($3 <> 'any' AND l.status::text = $3)
		      )
		  AND ($4 = 'any' OR l.channel::text = $4)
		ORDER BY q.score DESC NULLS LAST, l.updated_at DESC
		LIMIT $5`,
		userID, nullableCutoff(cutoff, hasCutoff), filter.Status, filter.Channel, filter.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("analytics get hot leads: %w", err)
	}
	defer rows.Close()

	dto := &HotLeadsDTO{Leads: []HotLeadDTO{}, LimitApplied: filter.Limit}
	for rows.Next() {
		var (
			row    HotLeadDTO
			reason *string
			total  int
		)
		if err := rows.Scan(&row.ID, &row.ContactName, &row.Channel, &row.Status, &row.LastActivityAt,
			&row.Score, &reason, &row.QualifiedAt, &total); err != nil {
			return nil, fmt.Errorf("analytics scan hot lead: %w", err)
		}
		if reason != nil {
			row.ScoreReason = *reason
		}
		dto.Leads = append(dto.Leads, row)
		dto.TotalMatching = total // identical on every row; window count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analytics hot leads rows iter: %w", err)
	}
	return dto, nil
}

// safeRatioInt divides totalMicro by count, returning 0 when count is
// non-positive. Integer-pure — the wire-mapping converts to float USD
// at the JSON boundary.
func safeRatioInt(totalMicro int64, count int) int64 {
	if count <= 0 {
		return 0
	}
	return totalMicro / int64(count)
}
