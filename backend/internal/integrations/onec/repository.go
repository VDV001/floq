package onec

import (
	"context"

	"github.com/daniil/floq/internal/integrations/onec/domain"
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

// InsertSyncRecord persists a ledger entry, returning inserted=false when the
// (user_id, external_id, external_type) dedup key already exists. This is the
// idempotency primitive: a replayed webhook resolves to a no-op insert rather
// than a duplicate row or an error.
func (r *Repository) InsertSyncRecord(ctx context.Context, rec *domain.SyncRecord) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO onec_sync_records
			(id, user_id, external_id, external_type, direction, kind, status, payload_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, external_id, external_type) DO NOTHING`,
		rec.ID, rec.UserID, rec.ExternalID, rec.ExternalType,
		string(rec.Direction), string(rec.Kind), string(rec.Status), rec.PayloadHash)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
