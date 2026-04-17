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
	q db.Querier
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{q: pool}
}

// NewRepositoryFromQuerier creates a Repository from any db.Querier (useful for testing).
func NewRepositoryFromQuerier(q db.Querier) *Repository {
	return &Repository{q: q}
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

	// Defaults.
	s := &domain.Settings{
		FullName:           fullName,
		Email:              email,
		IMAPPort:           "993",
		AIProvider:         "ollama",
		AIModel:            "gemma3:4b",
		NotifyTelegram:     true,
		AutoQualify:        true,
		AutoDraft:          true,
		AutoFollowup:       true,
		AutoFollowupDays:   2,
		AutoSendDelayMin:   5,
		AutoProspectToLead: true,
	}

	err = r.q.QueryRow(ctx,
		`SELECT telegram_bot_token, telegram_bot_active,
		        imap_host, imap_port, imap_user, imap_password,
		        resend_api_key,
		        smtp_host, smtp_port, smtp_user, smtp_password,
		        ai_provider, ai_model, ai_api_key,
		        notify_telegram, notify_email_digest,
		        auto_qualify, auto_draft, auto_send, auto_send_delay_min,
		        auto_followup, auto_followup_days, auto_prospect_to_lead, auto_verify_import
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
	)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("load settings: %w", err)
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
	err := r.q.QueryRow(ctx,
		`SELECT imap_password FROM user_settings WHERE user_id = $1`, userID,
	).Scan(&pwd)
	if err != nil {
		return "", err
	}
	return pwd, nil
}
