package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/daniil/floq/internal/audit/domain"
)

// Compile-time check: Repository satisfies the domain ports.
var (
	_ domain.AuditRepository     = (*Repository)(nil)
	_ domain.RetentionRepository = (*Repository)(nil)
)

// Repository persists audit_log rows via pgx. Bulk inserts use
// pgx.CopyFrom so a buffered batch from the async recorder writes in
// one network round-trip and is committed atomically (all-or-nothing).
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

var auditLogColumns = []string{
	"id", "user_id", "lead_id", "prospect_id",
	"request_type", "provider", "model",
	"input_tokens", "output_tokens", "total_tokens",
	"cost_usd_micro", "latency_ms",
	"status", "error_message", "created_at",
}

func (r *Repository) Save(ctx context.Context, entries []*domain.Entry) error {
	if len(entries) == 0 {
		return nil
	}
	_, err := r.pool.CopyFrom(ctx,
		pgx.Identifier{"audit_log"},
		auditLogColumns,
		pgx.CopyFromSlice(len(entries), func(i int) ([]any, error) {
			e := entries[i]
			return []any{
				e.ID,
				e.UserID,
				nullableUUID(e.LeadID),
				nullableUUID(e.ProspectID),
				string(e.RequestType),
				e.Provider,
				e.Model,
				e.InputTokens,
				e.OutputTokens,
				e.TotalTokens,
				e.CostUSDMicro,
				e.LatencyMS,
				string(e.Status),
				nullableString(e.ErrorMessage),
				e.CreatedAt,
			}, nil
		}),
	)
	if err != nil {
		return fmt.Errorf("audit save: %w", err)
	}
	return nil
}

// CostSummary aggregates audit_log rows for one user over the half-
// open interval [from, to). Two grouped queries (by request_type, by
// model) — totals are derived from the request-type breakdown to
// avoid a third round-trip and stay consistent by construction.
//
// Breakdowns are ordered by USDMicro DESC so the operator dashboard
// shows the most expensive surface first. Empty slices are returned
// instead of nil so the JSON wire-shape stays predictable.
func (r *Repository) CostSummary(ctx context.Context, userID uuid.UUID, from, to time.Time) (*domain.CostSummary, error) {
	summary := &domain.CostSummary{
		ByRequestType: []domain.RequestTypeBreakdown{},
		ByModel:       []domain.ModelBreakdown{},
		PeriodFrom:    from,
		PeriodTo:      to,
	}

	rows, err := r.pool.Query(ctx,
		`SELECT request_type, COUNT(*), COALESCE(SUM(cost_usd_micro), 0),
		        COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0)
		 FROM audit_log
		 WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
		 GROUP BY request_type
		 ORDER BY SUM(cost_usd_micro) DESC`,
		userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("audit cost-summary by request_type: %w", err)
	}
	for rows.Next() {
		var b domain.RequestTypeBreakdown
		if err := rows.Scan(&b.RequestType, &b.Calls, &b.USDMicro, &b.InputTokens, &b.OutputTokens); err != nil {
			rows.Close()
			return nil, fmt.Errorf("audit cost-summary scan request_type: %w", err)
		}
		summary.ByRequestType = append(summary.ByRequestType, b)
		summary.TotalCalls += b.Calls
		summary.TotalUSDMicro += b.USDMicro
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit cost-summary by request_type rows: %w", err)
	}

	rows, err = r.pool.Query(ctx,
		`SELECT model, COUNT(*), COALESCE(SUM(cost_usd_micro), 0),
		        COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0)
		 FROM audit_log
		 WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
		 GROUP BY model
		 ORDER BY SUM(cost_usd_micro) DESC`,
		userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("audit cost-summary by model: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var b domain.ModelBreakdown
		if err := rows.Scan(&b.Model, &b.Calls, &b.USDMicro, &b.InputTokens, &b.OutputTokens); err != nil {
			return nil, fmt.Errorf("audit cost-summary scan model: %w", err)
		}
		summary.ByModel = append(summary.ByModel, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("audit cost-summary by model rows: %w", err)
	}
	return summary, nil
}

// AggregateAndPurgeOlderThan implements domain.RetentionRepository. The
// roll-up and the delete run as ONE data-modifying CTE so they share a
// single snapshot: the rows fed into the aggregate are exactly the rows
// deleted, with no window in which a concurrent writer's row could be
// purged-but-not-counted. (In practice the recorder always stamps
// created_at=now(), so no fresh row is ever < threshold — but the CTE
// makes that safety structural rather than incidental.)
//
// Day bucketing uses the UTC calendar date of created_at so the result
// is independent of the connection's session timezone and lines up with
// the day-range filter the cost-summary read path applies to this table.
// ON CONFLICT accumulates onto an existing bucket, which keeps the
// operation idempotent and correct when a day is purged across multiple
// runs.
func (r *Repository) AggregateAndPurgeOlderThan(ctx context.Context, threshold time.Time) (int, error) {
	var purged int
	err := r.pool.QueryRow(ctx, `
WITH purged AS (
    DELETE FROM audit_log
    WHERE created_at < $1
    RETURNING (created_at AT TIME ZONE 'UTC')::date AS day,
              user_id, provider, model, request_type,
              cost_usd_micro, input_tokens, output_tokens
),
agg AS (
    SELECT day, user_id, provider, model, request_type,
           COUNT(*)                        AS total_calls,
           COALESCE(SUM(cost_usd_micro), 0) AS total_cost,
           COALESCE(SUM(input_tokens), 0)   AS total_in,
           COALESCE(SUM(output_tokens), 0)  AS total_out
    FROM purged
    GROUP BY day, user_id, provider, model, request_type
),
ins AS (
    INSERT INTO audit_log_daily AS d
        (day, user_id, provider, model, request_type,
         total_calls, total_cost_usd_micro, total_input_tokens, total_output_tokens)
    SELECT day, user_id, provider, model, request_type,
           total_calls, total_cost, total_in, total_out
    FROM agg
    ON CONFLICT (day, user_id, provider, model, request_type) DO UPDATE SET
        total_calls          = d.total_calls          + EXCLUDED.total_calls,
        total_cost_usd_micro = d.total_cost_usd_micro + EXCLUDED.total_cost_usd_micro,
        total_input_tokens   = d.total_input_tokens   + EXCLUDED.total_input_tokens,
        total_output_tokens  = d.total_output_tokens  + EXCLUDED.total_output_tokens
)
SELECT COUNT(*) FROM purged`, threshold).Scan(&purged)
	if err != nil {
		return 0, fmt.Errorf("audit retention aggregate-and-purge: %w", err)
	}
	return purged, nil
}

// nullableUUID converts an optional UUID into the form pgx expects for
// a nullable column: a typed nil yields SQL NULL, a non-nil pointer
// passes the value through.
func nullableUUID(u *uuid.UUID) any {
	if u == nil {
		return nil
	}
	return *u
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
