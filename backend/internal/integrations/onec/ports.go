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
	// AlreadyProcessed is true when a dedup hit found a record whose domain
	// action already succeeded (status 'processed'). When false on a dedup hit,
	// the event was recorded but never applied — reconciliation must re-apply it.
	AlreadyProcessed bool
}

// SyncStore persists ledger entries. Declared in the consumer package
// (usecase side) per DIP — the postgres Repository satisfies it.
type SyncStore interface {
	// InsertSyncRecord saves rec idempotently: a replayed dedup key yields
	// Inserted=false, and PayloadDrifted=true if the stored payload differs.
	InsertSyncRecord(ctx context.Context, rec *domain.SyncRecord) (InsertOutcome, error)
	// MarkProcessed flips the record's status to 'processed' once its domain
	// action has succeeded, so a later replay/reconciliation does not re-apply it.
	MarkProcessed(ctx context.Context, rec *domain.SyncRecord) error
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

// OneCReader reads recent events back from a tenant's 1C endpoint, used by
// reconciliation (#109) to re-apply events whose webhook was lost. The
// HTTP/OData implementation satisfies it. Kind may be empty per event — it is
// resolved from the mapping downstream, same as the webhook path.
type OneCReader interface {
	ListEvents(ctx context.Context, creds *domain.OutboundCredentials) ([]RawInboundEvent, error)
}

// OutboundStore resolves a tenant's outbound connection and persists the push
// ledger. The postgres Repository satisfies it. GetOutboundCredentials returns
// ErrOutboundNotConfigured when the tenant has no usable 1C endpoint.
type OutboundStore interface {
	GetOutboundCredentials(ctx context.Context, userID uuid.UUID) (*domain.OutboundCredentials, error)
	OutboundProcessedExists(ctx context.Context, userID uuid.UUID, externalID, externalType string) (bool, error)
	UpsertOutboundRecord(ctx context.Context, rec *domain.SyncRecord) error
}

// OneCClient pushes objects to a tenant's 1C endpoint. The HTTP/OData
// implementation (client.go) satisfies it; the usecase depends on this port so
// it can be faked in unit tests. CreateCounterparty returns the 1C-assigned
// reference (Ref_Key) on success — empty is allowed when 1C accepts the create
// but returns no body.
type OneCClient interface {
	CreateCounterparty(ctx context.Context, creds *domain.OutboundCredentials, draft *domain.CounterpartyDraft) (externalRef string, err error)
}

// ConfigStore persists a user's editable 1C credentials config (#110). The
// postgres Repository satisfies it. GetCredentialsConfig returns found=false
// when the user has no row yet (→ the usecase serves defaults); unlike
// GetOutboundCredentials it returns the row regardless of is_active/base_url.
type ConfigStore interface {
	GetCredentialsConfig(ctx context.Context, userID uuid.UUID) (*domain.CredentialsConfig, bool, error)
	UpsertCredentialsConfig(ctx context.Context, userID uuid.UUID, cfg *domain.CredentialsConfig) error
}

// MappingConfigStore reads/writes a user's full mapping config (active or not)
// for the settings editor. Satisfied by the Repository via the existing
// GetMappingConfig/SaveMappingConfig — distinct from inbound MappingStore,
// which only loads the ACTIVE config.
type MappingConfigStore interface {
	GetMappingConfig(ctx context.Context, userID uuid.UUID) (*domain.MappingConfig, error)
	SaveMappingConfig(ctx context.Context, cfg *domain.MappingConfig, isActive bool) error
}

// SecretGenerator produces a high-entropy webhook secret. The crypto/rand
// implementation lives in cmd/server (infra), so generation stays out of the
// usecase/domain and is fakeable in tests.
type SecretGenerator interface {
	WebhookSecret() (string, error)
}

// ConnectionTester probes a tenant's 1C endpoint to verify reachability and
// credentials for the settings "test connection" action (#110). Kept separate
// from OneCClient (ISP): the config usecase depends only on the probe, and
// adding it here doesn't disturb the outbound OneCClient fakes. The HTTP/OData
// implementation (client.go) satisfies it.
type ConnectionTester interface {
	TestConnection(ctx context.Context, creds *domain.OutboundCredentials) error
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
