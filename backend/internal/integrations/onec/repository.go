package onec

import (
	"context"
	"errors"

	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the postgres-backed store for the 1C sync ledger.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Repository over the given pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// InsertSyncRecord persists a ledger entry idempotently. On the dedup key
// (user_id, external_id, external_type) it inserts at most once; a replay is a
// no-op (Inserted=false). When deduped, it compares the stored payload hash to
// detect drift — a 1C document re-sent with changed content — so the caller can
// surface it instead of silently dropping a real update.
func (r *Repository) InsertSyncRecord(ctx context.Context, rec *domain.SyncRecord) (InsertOutcome, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO onec_sync_records
			(id, user_id, external_id, external_type, direction, kind, status, payload_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, external_id, external_type) DO NOTHING`,
		rec.ID, rec.UserID, rec.ExternalID, rec.ExternalType,
		string(rec.Direction), string(rec.Kind), string(rec.Status), rec.PayloadHash)
	if err != nil {
		return InsertOutcome{}, err
	}
	if tag.RowsAffected() == 1 {
		return InsertOutcome{Inserted: true}, nil
	}

	// Dedup hit: read the stored payload hash (drift) and status (whether the
	// domain action already succeeded — drives whether reconciliation re-applies).
	var storedHash, storedStatus string
	err = r.pool.QueryRow(ctx, `
		SELECT payload_hash, status FROM onec_sync_records
		WHERE user_id = $1 AND external_id = $2 AND external_type = $3`,
		rec.UserID, rec.ExternalID, rec.ExternalType).Scan(&storedHash, &storedStatus)
	if err != nil {
		return InsertOutcome{}, err
	}
	return InsertOutcome{
		Inserted:         false,
		PayloadDrifted:   storedHash != rec.PayloadHash,
		AlreadyProcessed: storedStatus == string(domain.SyncStatusProcessed),
	}, nil
}

// MarkProcessed flips an inbound ledger entry to 'processed' after its domain
// action succeeded. Keyed on the dedup tuple so it targets the row InsertSyncRecord
// wrote. Idempotent — re-marking an already-processed row is a no-op.
func (r *Repository) MarkProcessed(ctx context.Context, rec *domain.SyncRecord) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE onec_sync_records SET status = $4
		WHERE user_id = $1 AND external_id = $2 AND external_type = $3`,
		rec.UserID, rec.ExternalID, rec.ExternalType, string(domain.SyncStatusProcessed))
	return err
}

// UserIDByWebhookSecret resolves the owning user from a webhook secret. Only
// active credentials with a non-empty secret match, so a blank secret never
// authenticates. found=false when no row matches.
func (r *Repository) UserIDByWebhookSecret(ctx context.Context, secret string) (uuid.UUID, bool, error) {
	if secret == "" {
		return uuid.Nil, false, nil
	}
	var userID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT user_id FROM onec_credentials
		WHERE webhook_secret = $1 AND is_active = TRUE`, secret).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return userID, true, nil
}
