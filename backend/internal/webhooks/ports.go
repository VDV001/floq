package webhooks

import (
	"context"

	"github.com/daniil/floq/internal/webhooks/domain"
	"github.com/google/uuid"
)

// Store persists webhook endpoints and the delivery outbox. Declared in the
// consumer (usecase) per DIP; the pgx implementation is *Repository in this
// package, injected from the composition root.
type Store interface {
	// CreateEndpoint inserts a new subscription.
	CreateEndpoint(ctx context.Context, e *domain.WebhookEndpoint) error
	// ListEndpoints returns a user's endpoints (tenant-scoped), newest first.
	ListEndpoints(ctx context.Context, userID uuid.UUID) ([]*domain.WebhookEndpoint, error)
	// GetEndpoint loads one endpoint by ID (not tenant-scoped; the usecase
	// checks ownership). found=false when it does not exist.
	GetEndpoint(ctx context.Context, id uuid.UUID) (*domain.WebhookEndpoint, bool, error)
	// DeleteEndpoint removes an endpoint by ID.
	DeleteEndpoint(ctx context.Context, id uuid.UUID) error

	// EnqueueDelivery appends a pending delivery to the outbox.
	EnqueueDelivery(ctx context.Context, d *domain.WebhookDelivery) error
	// ClaimDueDeliveries returns up to limit pending deliveries whose backoff
	// has elapsed and whose attempts are below maxAttempts.
	ClaimDueDeliveries(ctx context.Context, limit, maxAttempts int) ([]*domain.WebhookDelivery, error)
	// SaveDelivery persists the outcome of a delivery attempt.
	SaveDelivery(ctx context.Context, d *domain.WebhookDelivery) error
}

// DeliveryClient POSTs a signed payload to an endpoint URL over an SSRF-hardened
// transport. Declared locally (DIP); the guarded-HTTP adapter implements it.
// It returns the HTTP status (0 on transport failure) and an error for any
// non-2xx or transport problem so the usecase can record and retry.
type DeliveryClient interface {
	Deliver(ctx context.Context, url string, payload []byte, signature string) (httpStatus int, err error)
}

// DeliveryObserver is the metrics seam: the composition root injects an adapter
// over the Prometheus registry. A nil observer disables instrumentation.
type DeliveryObserver interface {
	OnWebhookDelivery(eventType string, success bool)
}
