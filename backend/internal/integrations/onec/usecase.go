package onec

import (
	"context"
	"log/slog"

	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/google/uuid"
)

// UseCase orchestrates inbound 1C event handling: build a ledger record,
// persist it idempotently, and (later, #107) apply the mapped domain action.
type UseCase struct {
	store  SyncStore
	logger *slog.Logger
}

// Option configures a UseCase.
type Option func(*UseCase)

// WithLogger sets the logger used for drift/anomaly reporting.
func WithLogger(l *slog.Logger) Option {
	return func(u *UseCase) { u.logger = l }
}

// NewUseCase wires the use case over a SyncStore.
func NewUseCase(store SyncStore, opts ...Option) *UseCase {
	u := &UseCase{store: store, logger: slog.Default()}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

// ProcessResult reports the outcome of handling one inbound event.
type ProcessResult struct {
	// Deduped is true when the event was already seen and this call was a
	// no-op — the webhook handler still returns 200 for replays.
	Deduped bool
	// PayloadDrifted is true when a replay arrived with changed content. The
	// event is still deduped (not re-applied), but the drift is logged so a
	// real update is not lost without a trace.
	PayloadDrifted bool
}

// ProcessInboundEvent records a validated 1C event for the given user. A
// replay (same external id/type) is a no-op reported via Deduped. Mapping the
// event onto a domain action (move lead, upsert prospect) is out of scope here
// and handled in #107 — this step only guarantees idempotent capture.
func (u *UseCase) ProcessInboundEvent(ctx context.Context, userID uuid.UUID, ev *domain.ExternalEvent) (ProcessResult, error) {
	rec, err := domain.NewSyncRecord(userID, ev, domain.DirectionInbound)
	if err != nil {
		return ProcessResult{}, err
	}
	out, err := u.store.InsertSyncRecord(ctx, rec)
	if err != nil {
		return ProcessResult{}, err
	}
	if out.PayloadDrifted {
		u.logger.Warn("onec: replayed event arrived with changed payload; not re-applied",
			"user_id", userID,
			"external_id", ev.ExternalID,
			"external_type", ev.ExternalType,
			"kind", ev.Kind.String())
	}
	return ProcessResult{Deduped: !out.Inserted, PayloadDrifted: out.PayloadDrifted}, nil
}
