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

// Compile-time check that Repository implements domain.Repository.
var _ domain.Repository = (*Repository)(nil)

type Repository struct {
	q      db.Querier
	cipher SecretCipher
}

func NewRepository(pool *pgxpool.Pool, cipher SecretCipher) *Repository {
	return &Repository{q: pool, cipher: cipher}
}

// NewRepositoryFromQuerier creates a Repository from any db.Querier (useful for testing).
func NewRepositoryFromQuerier(q db.Querier, cipher SecretCipher) *Repository {
	return &Repository{q: q, cipher: cipher}
}

func (r *Repository) GetSettings(ctx context.Context, userID uuid.UUID) (*domain.Settings, error) {
	// Get profile from users table.
	var fullName, email string
	err := r.q.QueryRow(ctx,
		`SELECT full_name, email FROM users WHERE id = $1`, userID,
	).Scan(&fullName, &email)
	if err != nil {
		return nil, fmt.Errorf("load user profile: %w", err)
	}

	// Defaults applied when no user_settings row exists yet (pgx.ErrNoRows
	// path below). AIStyleCheckEnabled (migration 025) and
	// AggregatedInboxView (migration 027) both have DEFAULT TRUE in SQL —
	// keep the two in sync here if you change the policy.
	s := &domain.Settings{
		FullName:            fullName,
		Email:               email,
		IMAPPort:            "993",
		AIProvider:          "ollama",
		AIModel:             "gemma3:4b",
		AIStyleCheckEnabled: true,
		AggregatedInboxView: true,
		NotifyTelegram:      true,
		AutoQualify:         true,
		AutoDraft:           true,
		AutoFollowup:        true,
		AutoFollowupDays:    2,
		AutoSendDelayMin:    5,
		AutoProspectToLead:  true,
	}

	// Encrypted secret columns are appended at the end of the projection so
	// the legacy plaintext column ordering stays stable. Each is decrypted
	// below, preferring the ciphertext over the (possibly stale) plaintext.
	var (
		ttEnc, ttNonce       []byte
		imapEnc, imapNonce   []byte
		resendEnc, resendNon []byte
		smtpEnc, smtpNonce   []byte
		aiEnc, aiNonce       []byte
	)
	err = r.q.QueryRow(ctx,
		`SELECT telegram_bot_token, telegram_bot_active,
		        imap_host, imap_port, imap_user, imap_password,
		        resend_api_key,
		        smtp_host, smtp_port, smtp_user, smtp_password,
		        ai_provider, ai_model, ai_api_key,
		        notify_telegram, notify_email_digest,
		        auto_qualify, auto_draft, auto_send, auto_send_delay_min,
		        auto_followup, auto_followup_days, auto_prospect_to_lead, auto_verify_import,
		        ai_style_check_enabled,
		        aggregated_inbox_view,
		        telegram_bot_token_enc, telegram_bot_token_nonce,
		        imap_password_enc, imap_password_nonce,
		        resend_api_key_enc, resend_api_key_nonce,
		        smtp_password_enc, smtp_password_nonce,
		        ai_api_key_enc, ai_api_key_nonce
		 FROM user_settings WHERE user_id = $1`, userID,
	).Scan(
		&s.TelegramBotToken, &s.TelegramBotActive,
		&s.IMAPHost, &s.IMAPPort, &s.IMAPUser, &s.IMAPPassword,
		&s.ResendAPIKey,
		&s.SMTPHost, &s.SMTPPort, &s.SMTPUser, &s.SMTPPassword,
		&s.AIProvider, &s.AIModel, &s.AIAPIKey,
		&s.NotifyTelegram, &s.NotifyEmailDigest,
		&s.AutoQualify, &s.AutoDraft, &s.AutoSend, &s.AutoSendDelayMin,
		&s.AutoFollowup, &s.AutoFollowupDays, &s.AutoProspectToLead, &s.AutoVerifyImport,
		&s.AIStyleCheckEnabled,
		&s.AggregatedInboxView,
		&ttEnc, &ttNonce,
		&imapEnc, &imapNonce,
		&resendEnc, &resendNon,
		&smtpEnc, &smtpNonce,
		&aiEnc, &aiNonce,
	)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("load settings: %w", err)
	}

	for _, sec := range []struct {
		enc, nonce []byte
		field      *string
	}{
		{ttEnc, ttNonce, &s.TelegramBotToken},
		{imapEnc, imapNonce, &s.IMAPPassword},
		{resendEnc, resendNon, &s.ResendAPIKey},
		{smtpEnc, smtpNonce, &s.SMTPPassword},
		{aiEnc, aiNonce, &s.AIAPIKey},
	} {
		plain, derr := decryptOrFallback(r.cipher, sec.enc, sec.nonce, *sec.field)
		if derr != nil {
			return nil, fmt.Errorf("decrypt secret: %w", derr)
		}
		*sec.field = plain
	}

	return s, nil
}

func (r *Repository) UpdateSettings(ctx context.Context, userID uuid.UUID, fields map[string]any) error {
	if len(fields) == 0 {
		return fmt.Errorf("no fields to update")
	}

	// Build SQL: INSERT ... ON CONFLICT DO UPDATE SET ...
	// $1 = user_id, then $2..$N for column values.
	insertCols := "user_id"
	insertVals := "$1"
	updateSet := "updated_at = NOW()"
	args := []any{userID}

	i := 2
	for name, val := range fields {
		// Secret columns are encrypted at this boundary and stored in their
		// <col>_enc/<col>_nonce byte columns; the legacy plaintext column is
		// never written (it survives only for read-fallback until migration
		// 038). A non-string value for a secret column is a programming
		// error — settings secrets are always strings.
		if secretColumns[name] {
			plaintext, ok := val.(string)
			if !ok {
				return fmt.Errorf("secret column %q must be a string, got %T", name, val)
			}
			ciphertext, nonce, err := r.cipher.Encrypt(plaintext)
			if err != nil {
				return fmt.Errorf("encrypt %s: %w", name, err)
			}
			encCol, nonceCol := name+"_enc", name+"_nonce"
			insertCols += fmt.Sprintf(", %s, %s", encCol, nonceCol)
			insertVals += fmt.Sprintf(", $%d, $%d", i, i+1)
			updateSet += fmt.Sprintf(", %s = $%d, %s = $%d", encCol, i, nonceCol, i+1)
			args = append(args, ciphertext, nonce)
			i += 2
			continue
		}
		insertCols += fmt.Sprintf(", %s", name)
		insertVals += fmt.Sprintf(", $%d", i)
		updateSet += fmt.Sprintf(", %s = $%d", name, i)
		args = append(args, val)
		i++
	}

	query := fmt.Sprintf(
		`INSERT INTO user_settings (%s) VALUES (%s)
		 ON CONFLICT (user_id) DO UPDATE SET %s`,
		insertCols, insertVals, updateSet,
	)

	_, err := r.q.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}

// GetStoredIMAPPassword reads the stored IMAP password for test-imap flow.
// This is an extra method on the concrete type, not part of the domain interface.
func (r *Repository) GetStoredIMAPPassword(ctx context.Context, userID uuid.UUID) (string, error) {
	var pwd string
	var enc, nonce []byte
	err := r.q.QueryRow(ctx,
		`SELECT imap_password, imap_password_enc, imap_password_nonce
		 FROM user_settings WHERE user_id = $1`, userID,
	).Scan(&pwd, &enc, &nonce)
	if err != nil {
		return "", err
	}
	return decryptOrFallback(r.cipher, enc, nonce, pwd)
}
