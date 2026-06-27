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

// SendGuard is the agent-security-defaults layer-3 validator for outbound
// sends (channel allowlist, recipient schema, mass-send threshold). Declared
// here in the consumer per DIP; the security.OutboundGuard implementation is
// injected via an adapter from the composition root, so the outbound package
// carries no dependency on internal/ai/security. Returns (allowed, reason).
type SendGuard interface {
	CheckBatch(size int) (bool, string)
	CheckRecipient(channel, recipient string) (bool, string)
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
	// CountPendingDispatch returns how many outbound messages for the given
	// (prospect, sequence) run are still awaiting dispatch — status draft or
	// approved (see OutboundStatus.IsPendingDispatch). Zero means the run has
	// finished sending; the sender uses this to detect sequence completion.
	CountPendingDispatch(ctx context.Context, prospectID, sequenceID uuid.UUID) (int, error)
}

// SequenceCompletion identifies a prospect's sequence run that has just finished
// sending — its last message was dispatched and none remain pending.
type SequenceCompletion struct {
	UserID     uuid.UUID
	ProspectID uuid.UUID
	SequenceID uuid.UUID
}

// TxManager runs fn within a database transaction, exposing it through the
// context so transaction-aware repositories (and the sequence-completion
// emitter) join it via db.ConnFromCtx. Satisfied by *db.TxManager. Drives the
// #199 transactional outbox for sequence.completed: the dispatch's sent/bounced
// mark and the completion enqueue commit together. A nil TxManager (or emitter)
// falls back to the legacy post-commit observer path.
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// SequenceCompletionEmitter writes sequence.completed transactionally, inside
// the dispatch transaction that marked the run's last message sent/bounced
// (#199) — unlike SequenceCompletionObserver which fires post-commit. A non-nil
// error aborts that transaction (fail-closed); the un-marked message is re-sent
// on the next tick, which is safe on the idempotent Resend path and within the
// pre-existing accepted duplicate window on the SMTP path. Implemented in the
// composition root.
type SequenceCompletionEmitter interface {
	EmitSequenceCompleted(ctx context.Context, ev SequenceCompletion) error
}

// ProspectLookup reads prospect data and the suppression list — everything the
// outbound context needs from the prospects context to make a send decision.
type ProspectLookup interface {
	GetProspect(ctx context.Context, id uuid.UUID) (*prospectsdomain.Prospect, error)
	UpdateVerification(ctx context.Context, id uuid.UUID, status prospectsdomain.VerifyStatus, score int, details string, verifiedAt time.Time) error
	// IsSuppressed reports whether address is on the suppression list for the
	// user on the given channel — the hard pre-check ahead of consent.
	IsSuppressed(ctx context.Context, userID uuid.UUID, channel prospectsdomain.SuppressionChannel, address string) (bool, error)
}

// TelegramSessionStore retrieves Telegram MTProto session data.
type TelegramSessionStore interface {
	GetSession(ctx context.Context, ownerID string) (phone string, sessionData []byte, err error)
}

// TelegramMessenger sends messages via personal Telegram account.
type TelegramMessenger interface {
	SendMessage(ctx context.Context, sessionData []byte, target, body string) error
}
