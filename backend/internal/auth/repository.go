package auth

import (
	"context"
	"time"

	"github.com/daniil/floq/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository implements UserRepository using PostgreSQL.
type Repository struct {
	q db.Querier
}

// NewRepository creates a new Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{q: pool}
}

// NewRepositoryFromQuerier creates a Repository from any db.Querier (useful for testing).
func NewRepositoryFromQuerier(q db.Querier) *Repository {
	return &Repository{q: q}
}

func (r *Repository) CreateUser(ctx context.Context, id uuid.UUID, email, passwordHash, fullName string, now time.Time) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		id, email, passwordHash, fullName, now, now)
	return err
}

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (uuid.UUID, string, error) {
	var userID uuid.UUID
	var passwordHash string
	err := r.q.QueryRow(ctx,
		`SELECT id, password_hash FROM users WHERE email = $1`, email).
		Scan(&userID, &passwordHash)
	return userID, passwordHash, err
}
