package tgclient

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles telegram_sessions table operations.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new telegram session repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// SaveSession upserts a telegram session for the given user.
func (r *Repository) SaveSession(ctx context.Context, userID, phone string, sessionData []byte) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO telegram_sessions (user_id, phone, session_data, is_active, updated_at)
		 VALUES ($1, $2, $3, true, NOW())
		 ON CONFLICT (user_id) DO UPDATE SET phone = $2, session_data = $3, is_active = true, updated_at = NOW()`,
		userID, phone, sessionData)
	if err != nil {
		return fmt.Errorf("save telegram session: %w", err)
	}
	return nil
}

// GetSession retrieves the telegram session for a user.
// Returns empty phone and nil data if no session exists.
func (r *Repository) GetSession(ctx context.Context, userID string) (phone string, sessionData []byte, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT phone, session_data FROM telegram_sessions WHERE user_id = $1 AND is_active = true`,
		userID).Scan(&phone, &sessionData)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("get telegram session: %w", err)
	}
	return phone, sessionData, nil
}

// DeleteSession removes the telegram session for a user.
func (r *Repository) DeleteSession(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM telegram_sessions WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete telegram session: %w", err)
	}
	return nil
}
