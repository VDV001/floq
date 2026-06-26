package settings

import (
	"context"
	"errors"
	"fmt"

	"github.com/daniil/floq/internal/db"
	"github.com/daniil/floq/internal/settings/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time check that Store implements domain.ConfigStore.
var _ domain.ConfigStore = (*Store)(nil)

// Store reads user settings from the database.
type Store struct {
	q      db.Querier
	cipher SecretCipher
}

func NewStore(pool *pgxpool.Pool, cipher SecretCipher) *Store {
	return &Store{q: pool, cipher: cipher}
}

// NewStoreFromQuerier creates a Store from any db.Querier (useful for testing).
func NewStoreFromQuerier(q db.Querier, cipher SecretCipher) *Store {
	return &Store{q: q, cipher: cipher}
}

// GetConfig reads settings for the given user. Returns zero-value UserConfig if no row exists.
func (s *Store) GetConfig(ctx context.Context, userID uuid.UUID) (*domain.UserConfig, error) {
	cfg := &domain.UserConfig{}
	// Encrypted secret columns appended at the end; decrypted below with a
	// plaintext fallback for rows not yet backfilled (see migration 037).
	var (
		resendEnc, resendNon []byte
		smtpEnc, smtpNonce   []byte
		aiEnc, aiNonce       []byte
		imapEnc, imapNonce   []byte
		ttEnc, ttNonce       []byte
	)
	err := s.q.QueryRow(ctx,
		`SELECT COALESCE(resend_api_key, ''),
		        COALESCE(smtp_host, ''), COALESCE(smtp_port, '465'), COALESCE(smtp_user, ''), COALESCE(smtp_password, ''),
		        COALESCE(ai_provider, ''), COALESCE(ai_model, ''), COALESCE(ai_api_key, ''),
		        COALESCE(imap_host, ''), COALESCE(imap_port, '993'), COALESCE(imap_user, ''), COALESCE(imap_password, ''),
		        COALESCE(telegram_bot_token, ''),
		        resend_api_key_enc, resend_api_key_nonce,
		        smtp_password_enc, smtp_password_nonce,
		        ai_api_key_enc, ai_api_key_nonce,
		        imap_password_enc, imap_password_nonce,
		        telegram_bot_token_enc, telegram_bot_token_nonce
		 FROM user_settings WHERE user_id = $1`, userID,
	).Scan(
		&cfg.ResendAPIKey,
		&cfg.SMTPHost, &cfg.SMTPPort, &cfg.SMTPUser, &cfg.SMTPPassword,
		&cfg.AIProvider, &cfg.AIModel, &cfg.AIAPIKey,
		&cfg.IMAPHost, &cfg.IMAPPort, &cfg.IMAPUser, &cfg.IMAPPassword,
		&cfg.TelegramBotToken,
		&resendEnc, &resendNon,
		&smtpEnc, &smtpNonce,
		&aiEnc, &aiNonce,
		&imapEnc, &imapNonce,
		&ttEnc, &ttNonce,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		// No row = empty config, not an error for callers.
		return &domain.UserConfig{}, nil
	}
	if err != nil {
		// A real DB or decode error must surface, not masquerade as an empty
		// config — otherwise background senders silently fall back to .env.
		return nil, fmt.Errorf("load config: %w", err)
	}

	for _, sec := range []struct {
		enc, nonce []byte
		field      *string
	}{
		{resendEnc, resendNon, &cfg.ResendAPIKey},
		{smtpEnc, smtpNonce, &cfg.SMTPPassword},
		{aiEnc, aiNonce, &cfg.AIAPIKey},
		{imapEnc, imapNonce, &cfg.IMAPPassword},
		{ttEnc, ttNonce, &cfg.TelegramBotToken},
	} {
		plain, derr := decryptOrFallback(s.cipher, sec.enc, sec.nonce, *sec.field)
		if derr != nil {
			return nil, fmt.Errorf("decrypt secret: %w", derr)
		}
		*sec.field = plain
	}
	return cfg, nil
}
