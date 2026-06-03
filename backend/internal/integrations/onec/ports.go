package onec

import (
	"context"

	"github.com/daniil/floq/internal/integrations/onec/domain"
)

// SyncStore persists ledger entries. Declared in the consumer package
// (usecase side) per DIP — the postgres Repository satisfies it.
type SyncStore interface {
	// InsertSyncRecord saves rec, returning inserted=false when the dedup
	// key already exists (idempotent replay).
	InsertSyncRecord(ctx context.Context, rec *domain.SyncRecord) (bool, error)
}
