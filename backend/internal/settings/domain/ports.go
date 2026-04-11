package domain

import (
	"context"

	"github.com/google/uuid"
)

// Repository provides persistence operations for user settings.
type Repository interface {
	GetSettings(ctx context.Context, userID uuid.UUID) (*Settings, error)
	UpdateSettings(ctx context.Context, userID uuid.UUID, fields map[string]any) error
	GetStoredIMAPPassword(ctx context.Context, userID uuid.UUID) (string, error)
}

// ConfigStore provides read-only access to user configuration needed by
// background services.
type ConfigStore interface {
	GetConfig(ctx context.Context, userID uuid.UUID) (*UserConfig, error)
}

// TelegramTokenValidator validates a Telegram bot token.
type TelegramTokenValidator interface {
	Validate(token string) error
}
