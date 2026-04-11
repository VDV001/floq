package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository implements UserRepository using PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository backed by the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateUser(ctx context.Context, id uuid.UUID, email, passwordHash, fullName string, now time.Time) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		id, email, passwordHash, fullName, now, now)
	return err
}

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (uuid.UUID, string, error) {
	var userID uuid.UUID
	var passwordHash string
	err := r.pool.QueryRow(ctx,
		`SELECT id, password_hash FROM users WHERE email = $1`, email).
		Scan(&userID, &passwordHash)
	return userID, passwordHash, err
}
