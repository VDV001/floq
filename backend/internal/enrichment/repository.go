package enrichment

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/daniil/floq/internal/enrichment/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the pgx-backed Store implementation.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs the enrichment repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

var _ Store = (*Repository)(nil)

// UpsertPending inserts a pending row for (user, domain); a pre-existing row is
// left untouched (dedup) — the worker refreshes it via its TTL.
func (r *Repository) UpsertPending(ctx context.Context, e *domain.CompanyEnrichment) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO company_enrichment (id, user_id, domain, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, domain) DO NOTHING`,
		e.ID, e.UserID, e.Domain.String(), e.Status.String(), e.CreatedAt, e.UpdatedAt)
	if err != nil {
		return fmt.Errorf("enrichment: upsert pending: %w", err)
	}
	return nil
}

// ClaimDue returns rows that are due for (re)processing — pending, failed
// (for retry), or enriched-and-expired (for refresh) — with attempts below
// maxAttempts, oldest first. The attempts cap is what bounds retries of a
// persistently-failing domain.
//
// Phase 1 runs a single worker (the EnrichmentCron), so this plain SELECT needs
// no cross-instance claim/lease; multi-instance exclusivity (a processing lease)
// is a Phase-2 hardening.
func (r *Repository) ClaimDue(ctx context.Context, limit, maxAttempts int) ([]*domain.CompanyEnrichment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, domain, status, profile, error, attempts, enriched_at, expires_at, created_at, updated_at
		FROM company_enrichment
		WHERE attempts < $2
		  AND (status = 'pending'
		       OR status = 'failed'
		       OR (status = 'enriched' AND expires_at < now()))
		ORDER BY updated_at
		LIMIT $1`, limit, maxAttempts)
	if err != nil {
		return nil, fmt.Errorf("enrichment: claim due: %w", err)
	}
	defer rows.Close()

	var out []*domain.CompanyEnrichment
	for rows.Next() {
		e, err := scanEnrichment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Save persists the processed state of a record.
func (r *Repository) Save(ctx context.Context, e *domain.CompanyEnrichment) error {
	profileJSON, err := json.Marshal(e.Profile)
	if err != nil {
		return fmt.Errorf("enrichment: marshal profile: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE company_enrichment
		SET status = $2, profile = $3, error = $4, attempts = $5,
		    enriched_at = $6, expires_at = $7, updated_at = $8
		WHERE id = $1`,
		e.ID, e.Status.String(), profileJSON, e.Error, e.Attempts,
		e.EnrichedAt, e.ExpiresAt, e.UpdatedAt)
	if err != nil {
		return fmt.Errorf("enrichment: save: %w", err)
	}
	return nil
}

// Get returns the enrichment for (user, domain), tenant-scoped.
func (r *Repository) Get(ctx context.Context, userID uuid.UUID, domainName string) (*domain.CompanyEnrichment, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, domain, status, profile, error, attempts, enriched_at, expires_at, created_at, updated_at
		FROM company_enrichment
		WHERE user_id = $1 AND domain = $2`, userID, domainName)
	e, err := scanEnrichment(row)
	if err == pgx.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return e, true, nil
}

// scanner is satisfied by both pgx.Row and pgx.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanEnrichment(s scanner) (*domain.CompanyEnrichment, error) {
	var (
		e           domain.CompanyEnrichment
		domainName  string
		status      string
		profileJSON []byte
	)
	if err := s.Scan(&e.ID, &e.UserID, &domainName, &status, &profileJSON,
		&e.Error, &e.Attempts, &e.EnrichedAt, &e.ExpiresAt, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return nil, err
	}
	e.Domain = domain.DomainFromStorage(domainName)
	e.Status = domain.Status(status)
	if len(profileJSON) > 0 {
		if err := json.Unmarshal(profileJSON, &e.Profile); err != nil {
			return nil, fmt.Errorf("enrichment: unmarshal profile: %w", err)
		}
	}
	return &e, nil
}
