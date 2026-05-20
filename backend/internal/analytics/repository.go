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

// Compile-time check: Repository satisfies the port consumed by the
// usecase. Adding the assertion here means a signature drift breaks
// the build, not a runtime cast at wire-up time.
var _ SequenceStatsReader = (*Repository)(nil)

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
