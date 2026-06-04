package analytics

import (
	"context"
	"fmt"
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

// GetInboxFlow is stubbed pending the GREEN implementation — returns an
// empty read model so the integration tests compile and fail on the
// real assertions rather than panicking on a nil deref.
func (r *Repository) GetInboxFlow(ctx context.Context, userID uuid.UUID, from, to time.Time) (*InboxFlowDTO, error) {
	return &InboxFlowDTO{
		PeriodFrom: from,
		PeriodTo:   to,
		Leads:      LeadsBreakdownDTO{ByChannel: map[string]int{}, ByStatus: map[string]int{}},
	}, nil
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
		 WHERE user_id = $1 AND created_at >= $2 AND created_at < $3`,
		userID, from, to,
	).Scan(&dto.LeadsCount); err != nil {
		return nil, fmt.Errorf("analytics cost-ratios leads: %w", err)
	}

	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM (
			SELECT l.id FROM leads l
			JOIN qualifications q ON q.lead_id = l.id
			WHERE l.user_id = $1 AND l.created_at >= $2 AND l.created_at < $3
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
// an explicit status returns exactly that state.
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
