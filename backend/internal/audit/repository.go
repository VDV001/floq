package audit

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/daniil/floq/internal/audit/domain"
)

// Compile-time check: Repository satisfies the domain port.
var _ domain.AuditRepository = (*Repository)(nil)

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
