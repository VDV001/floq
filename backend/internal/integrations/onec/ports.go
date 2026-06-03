package onec

import (
	"context"

	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/google/uuid"
)

// SyncStore persists ledger entries. Declared in the consumer package
// (usecase side) per DIP — the postgres Repository satisfies it.
type SyncStore interface {
	// InsertSyncRecord saves rec, returning inserted=false when the dedup
	// key already exists (idempotent replay).
	InsertSyncRecord(ctx context.Context, rec *domain.SyncRecord) (bool, error)
}

// SecretResolver maps an inbound webhook secret to its owning user. The
// webhook carries no JWT (1C can't issue one), so the secret is the tenant
// credential. The postgres Repository satisfies it.
type SecretResolver interface {
	// UserIDByWebhookSecret returns the user whose active 1C credentials match
	// secret. found=false means no match (→ 401).
	UserIDByWebhookSecret(ctx context.Context, secret string) (uuid.UUID, bool, error)
}
