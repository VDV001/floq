package domain

import (
	"context"

	"github.com/google/uuid"
)

// Repository provides persistence operations for user settings.
type Repository interface {
	GetSettings(ctx context.Context, userID uuid.UUID) (*Settings, error)
	UpdateSettings(ctx context.Context, userID uuid.UUID, fields map[string]any) error
}

// ConfigStore provides read-only access to user configuration needed by
// background services.
type ConfigStore interface {
	GetConfig(ctx context.Context, userID uuid.UUID) (*UserConfig, error)
}
