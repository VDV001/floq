package onec

import (
	"context"

	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/google/uuid"
)

// UseCase orchestrates inbound 1C event handling: build a ledger record,
// persist it idempotently, and (later, #107) apply the mapped domain action.
type UseCase struct {
	store SyncStore
}

// NewUseCase wires the use case over a SyncStore.
func NewUseCase(store SyncStore) *UseCase {
	return &UseCase{store: store}
}

// ProcessResult reports the outcome of handling one inbound event.
type ProcessResult struct {
	// Deduped is true when the event was already seen and this call was a
	// no-op — the webhook handler still returns 200 for replays.
	Deduped bool
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
	inserted, err := u.store.InsertSyncRecord(ctx, rec)
	if err != nil {
		return ProcessResult{}, err
	}
	return ProcessResult{Deduped: !inserted}, nil
}
