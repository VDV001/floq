package onec

import (
	"context"
	"errors"

	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Compile-time checks: Repository satisfies the config-management ports.
var (
	_ ConfigStore        = (*Repository)(nil)
	_ MappingConfigStore = (*Repository)(nil)
)

// GetCredentialsConfig loads a user's editable 1C config — every field
// regardless of is_active or base_url. This is deliberately broader than
// GetOutboundCredentials (which only returns a usable, active connection): the
// settings UI must show a half-filled or disabled config too. found=false
// means the user has no row yet, so the usecase serves defaults.
func (r *Repository) GetCredentialsConfig(ctx context.Context, userID uuid.UUID) (*domain.CredentialsConfig, bool, error) {
	var baseURL, authType, authSecret, webhookSecret string
	var isActive bool
	err := r.pool.QueryRow(ctx, `
		SELECT base_url, auth_type, auth_secret, webhook_secret, is_active
		FROM onec_credentials WHERE user_id = $1`, userID).
		Scan(&baseURL, &authType, &authSecret, &webhookSecret, &isActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	cfg, err := domain.NewCredentialsConfig(baseURL, domain.AuthType(authType), authSecret, webhookSecret, isActive)
	if err != nil {
		return nil, false, err
	}
	return cfg, true, nil
}

// UpsertCredentialsConfig persists the full config (one row per user). The
// read-merge-write happens in the usecase, so this just writes the resulting
// validated VO; ON CONFLICT keeps the single-row-per-user invariant.
func (r *Repository) UpsertCredentialsConfig(ctx context.Context, userID uuid.UUID, cfg *domain.CredentialsConfig) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO onec_credentials
			(user_id, base_url, auth_type, auth_secret, webhook_secret, is_active, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			base_url       = EXCLUDED.base_url,
			auth_type      = EXCLUDED.auth_type,
			auth_secret    = EXCLUDED.auth_secret,
			webhook_secret = EXCLUDED.webhook_secret,
			is_active      = EXCLUDED.is_active,
			updated_at     = NOW()`,
		userID, cfg.BaseURL, string(cfg.AuthType), cfg.AuthSecret, cfg.WebhookSecret, cfg.IsActive)
	return err
}
