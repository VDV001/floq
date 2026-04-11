package outbound

import (
	"context"
	"time"

	"github.com/google/uuid"

	prospectsdomain "github.com/daniil/floq/internal/prospects/domain"
	seqdomain "github.com/daniil/floq/internal/sequences/domain"
	settingsdomain "github.com/daniil/floq/internal/settings/domain"
)

// ConfigStore retrieves user-level configuration (SMTP, API keys, etc.).
type ConfigStore interface {
	GetConfig(ctx context.Context, userID uuid.UUID) (*settingsdomain.UserConfig, error)
}

// OutboundRepository manages the outbound message queue.
type OutboundRepository interface {
	GetPendingSends(ctx context.Context) ([]seqdomain.OutboundMessage, error)
	MarkSent(ctx context.Context, id uuid.UUID) error
	MarkBounced(ctx context.Context, id uuid.UUID) error
}

// ProspectLookup reads and updates prospect data needed for sending.
type ProspectLookup interface {
	GetProspect(ctx context.Context, id uuid.UUID) (*prospectsdomain.Prospect, error)
	UpdateVerification(ctx context.Context, id uuid.UUID, status prospectsdomain.VerifyStatus, score int, details string, verifiedAt time.Time) error
}

// TelegramSessionStore retrieves Telegram MTProto session data.
type TelegramSessionStore interface {
	GetSession(ctx context.Context, ownerID string) (phone string, sessionData []byte, err error)
}

// TelegramMessenger sends messages via personal Telegram account.
type TelegramMessenger interface {
	SendMessage(ctx context.Context, sessionData []byte, target, body string) error
}
