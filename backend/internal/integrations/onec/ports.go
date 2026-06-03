package onec

import (
	"context"

	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/google/uuid"
)

// InsertOutcome reports the result of an idempotent ledger insert.
type InsertOutcome struct {
	// Inserted is true when a new row was written (false on a dedup hit).
	Inserted bool
	// PayloadDrifted is true when the event was deduped but arrived with a
	// different payload than the stored one — a replayed 1C document whose
	// content changed. The caller decides what to do (we log it); a silent
	// drop would lose a real update.
	PayloadDrifted bool
}

// SyncStore persists ledger entries. Declared in the consumer package
// (usecase side) per DIP — the postgres Repository satisfies it.
type SyncStore interface {
	// InsertSyncRecord saves rec idempotently: a replayed dedup key yields
	// Inserted=false, and PayloadDrifted=true if the stored payload differs.
	InsertSyncRecord(ctx context.Context, rec *domain.SyncRecord) (InsertOutcome, error)
}

// SecretResolver maps an inbound webhook secret to its owning user. The
// webhook carries no JWT (1C can't issue one), so the secret is the tenant
// credential. The postgres Repository satisfies it.
type SecretResolver interface {
	// UserIDByWebhookSecret returns the user whose active 1C credentials match
	// secret. found=false means no match (→ 401).
	UserIDByWebhookSecret(ctx context.Context, secret string) (uuid.UUID, bool, error)
}

// MappingStore loads a user's ACTIVE 1C→Floq mapping config for application.
// The postgres Repository satisfies it; returns ErrMappingNotFound when the
// user has no active config (so an inactive config genuinely disables mapping).
type MappingStore interface {
	GetActiveMappingConfig(ctx context.Context, userID uuid.UUID) (*domain.MappingConfig, error)
}

// EventApplier performs the domain action for a resolved 1C event. Implemented
// by a cross-context adapter (cmd/server/adapters.go) over leads/prospects —
// onec never imports those contexts directly. Actions that target an existing
// entity (payment/order/shipment) no-op when the counterparty is unknown; only
// counterparty-created upserts.
type EventApplier interface {
	HandlePayment(ctx context.Context, userID uuid.UUID, email string) error
	HandleCounterpartyCreated(ctx context.Context, userID uuid.UUID, email, name, company string) error
	HandleOrderStatus(ctx context.Context, userID uuid.UUID, email string) error
	HandleShipment(ctx context.Context, userID uuid.UUID, email string) error
}
