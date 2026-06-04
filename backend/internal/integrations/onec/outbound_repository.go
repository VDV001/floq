package onec

import (
	"context"
	"errors"

	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrOutboundNotConfigured is returned when a user has no usable outbound 1C
// connection — no credentials row, inactive, or a blank base URL. The outbound
// flow treats it as "1C push disabled for this tenant", not a hard error.
var ErrOutboundNotConfigured = errors.New("onec: outbound 1C not configured")

// GetOutboundCredentials loads a user's outbound connection. Only an active row
// with a non-empty base URL is usable; anything else yields
// ErrOutboundNotConfigured. The domain factory validates/normalises the result.
func (r *Repository) GetOutboundCredentials(ctx context.Context, userID uuid.UUID) (*domain.OutboundCredentials, error) {
	var baseURL, authType, authSecret string
	err := r.pool.QueryRow(ctx, `
		SELECT base_url, auth_type, auth_secret FROM onec_credentials
		WHERE user_id = $1 AND is_active = TRUE AND base_url <> ''`, userID).
		Scan(&baseURL, &authType, &authSecret)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOutboundNotConfigured
	}
	if err != nil {
		return nil, err
	}
	at, err := domain.ParseAuthType(authType)
	if err != nil {
		return nil, err
	}
	return domain.NewOutboundCredentials(baseURL, at, authSecret)
}

// UpsertOutboundRecord writes the ledger entry for an outbound push, keyed on
// the dedup tuple. A retry of the same (user, external_id, external_type) flips
// the stored status/kind in place rather than inserting a duplicate, so an
// earlier 'error' becomes 'processed' once a later push succeeds.
func (r *Repository) UpsertOutboundRecord(ctx context.Context, rec *domain.SyncRecord) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO onec_sync_records
			(id, user_id, external_id, external_type, direction, kind, status, payload_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, external_id, external_type) DO UPDATE
			SET status = EXCLUDED.status, kind = EXCLUDED.kind, direction = EXCLUDED.direction`,
		rec.ID, rec.UserID, rec.ExternalID, rec.ExternalType,
		string(rec.Direction), string(rec.Kind), string(rec.Status), rec.PayloadHash)
	return err
}

// ActiveOnecUserIDs lists every tenant with a usable 1C connection (active +
// non-empty base URL) — the targets reconciliation (#109) polls for missed
// events. Ordered for deterministic iteration.
func (r *Repository) ActiveOnecUserIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT user_id FROM onec_credentials
		WHERE is_active = TRUE AND base_url <> ''
		ORDER BY user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// OutboundProcessedExists reports whether the object was already pushed to 1C
// successfully (a 'processed' record for the dedup key). The outbound flow uses
// it to skip re-creating a counterparty; an 'error' record does NOT count, so a
// failed push is retried on the next trigger.
func (r *Repository) OutboundProcessedExists(ctx context.Context, userID uuid.UUID, externalID, externalType string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM onec_sync_records
			WHERE user_id = $1 AND external_id = $2 AND external_type = $3
			  AND direction = 'outbound' AND status = 'processed'
		)`, userID, externalID, externalType).Scan(&exists)
	return exists, err
}
