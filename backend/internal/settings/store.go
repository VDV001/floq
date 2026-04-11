package settings

import (
	"context"

	"github.com/daniil/floq/internal/settings/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time check that Store implements domain.ConfigStore.
var _ domain.ConfigStore = (*Store)(nil)

// Store reads user settings from the database.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// GetConfig reads settings for the given user. Returns zero-value UserConfig if no row exists.
func (s *Store) GetConfig(ctx context.Context, userID uuid.UUID) (*domain.UserConfig, error) {
	cfg := &domain.UserConfig{}
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(resend_api_key, ''),
		        COALESCE(smtp_host, ''), COALESCE(smtp_port, '465'), COALESCE(smtp_user, ''), COALESCE(smtp_password, ''),
		        COALESCE(ai_provider, ''), COALESCE(ai_model, ''), COALESCE(ai_api_key, ''),
		        COALESCE(imap_host, ''), COALESCE(imap_port, '993'), COALESCE(imap_user, ''), COALESCE(imap_password, ''),
		        COALESCE(telegram_bot_token, '')
		 FROM user_settings WHERE user_id = $1`, userID,
	).Scan(
		&cfg.ResendAPIKey,
		&cfg.SMTPHost, &cfg.SMTPPort, &cfg.SMTPUser, &cfg.SMTPPassword,
		&cfg.AIProvider, &cfg.AIModel, &cfg.AIAPIKey,
		&cfg.IMAPHost, &cfg.IMAPPort, &cfg.IMAPUser, &cfg.IMAPPassword,
		&cfg.TelegramBotToken,
	)
	if err != nil {
		// No row = empty config, not an error for callers
		return &domain.UserConfig{}, nil
	}
	return cfg, nil
}
