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
	// MarkSent persists sent status plus the caller-supplied sent_at
	// timestamp; see sequences/domain.Repository contract — the repo must
	// not fall back to DB-side NOW(). The clock is computed by the domain
	// entity's OutboundMessage.MarkSent before this call.
	MarkSent(ctx context.Context, id uuid.UUID, sentAt time.Time) error
	// MarkBounced persists the bounced terminal state; bouncedAt supplied
	// by the caller (the domain entity's OutboundMessage.MarkBounced) —
	// see sequences/domain.Repository contract for the rationale.
	MarkBounced(ctx context.Context, id uuid.UUID, bouncedAt time.Time) error
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
