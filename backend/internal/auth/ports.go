package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// UserRepository defines persistence operations for users.
type UserRepository interface {
	CreateUser(ctx context.Context, id uuid.UUID, email, passwordHash, fullName string, now time.Time) error
	FindUserByEmail(ctx context.Context, email string) (userID uuid.UUID, passwordHash string, err error)
}
