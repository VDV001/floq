package leads

import (
	"context"
	"errors"
	"fmt"

	"github.com/daniil/floq/internal/db"
	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time check that IdentityRepository satisfies the domain port.
var _ domain.IdentityRepository = (*IdentityRepository)(nil)

// IdentityRepository persists domain.Identity rows in the `identities`
// table and looks them up by each canonical identifier. Lookups are
// scoped to a user_id so identical handles owned by different users
// stay isolated.
type IdentityRepository struct {
	pool *pgxpool.Pool
}

// NewIdentityRepository wires the SQL-backed implementation.
func NewIdentityRepository(pool *pgxpool.Pool) *IdentityRepository {
	return &IdentityRepository{pool: pool}
}

// q returns the Querier bound to the current context (a pgx.Tx when
// the caller wrapped the call in db.TxManager.WithTx, otherwise the
// pool) — mirrors the pattern used by the existing Repository.
func (r *IdentityRepository) q(ctx context.Context) db.Querier {
	return db.ConnFromCtx(ctx, r.pool)
}

func (r *IdentityRepository) Save(ctx context.Context, id *domain.Identity) error {
	_, err := r.q(ctx).Exec(ctx,
		`INSERT INTO identities (id, user_id, email, phone, telegram_username, created_at)
		 VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6)`,
		id.ID, id.UserID, id.Email, id.Phone, id.TelegramUsername, id.CreatedAt)
	if err != nil {
		return fmt.Errorf("save identity: %w", err)
	}
	return nil
}

func (r *IdentityRepository) FindByEmail(ctx context.Context, userID uuid.UUID, email string) (*domain.Identity, error) {
	return r.findByColumn(ctx, userID, "email", email)
}

func (r *IdentityRepository) FindByPhone(ctx context.Context, userID uuid.UUID, phone string) (*domain.Identity, error) {
	return r.findByColumn(ctx, userID, "phone", phone)
}

func (r *IdentityRepository) FindByTelegramUsername(ctx context.Context, userID uuid.UUID, tg string) (*domain.Identity, error) {
	return r.findByColumn(ctx, userID, "telegram_username", tg)
}

// findByColumn factors the three FindByX queries — only the column name
// differs and it comes from a fixed allowlist controlled by the caller,
// so string interpolation here is safe (no user-supplied table/column).
func (r *IdentityRepository) findByColumn(ctx context.Context, userID uuid.UUID, column, value string) (*domain.Identity, error) {
	if value == "" {
		return nil, nil
	}
	query := fmt.Sprintf(
		`SELECT id, user_id, COALESCE(email, ''), COALESCE(phone, ''), COALESCE(telegram_username, ''), created_at
		 FROM identities WHERE user_id = $1 AND %s = $2`, column)

	var id domain.Identity
	err := r.q(ctx).QueryRow(ctx, query, userID, value).
		Scan(&id.ID, &id.UserID, &id.Email, &id.Phone, &id.TelegramUsername, &id.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find identity by %s: %w", column, err)
	}
	return &id, nil
}
