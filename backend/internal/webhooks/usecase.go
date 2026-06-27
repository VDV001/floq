// Package webhooks is the application layer for outgoing webhooks (#181):
// endpoint CRUD, event publication (fan-out to the outbox), and the delivery
// worker that POSTs signed payloads with retries. Business invariants live in
// internal/webhooks/domain; this package orchestrates ports.
package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/daniil/floq/internal/webhooks/domain"
	"github.com/google/uuid"
)

// ErrEndpointNotFound is returned when an endpoint does not exist or is not
// owned by the caller. Both cases collapse to one error so the API cannot be
// used to enumerate other tenants' endpoint IDs (mirrors the leads ownership
// pattern: not-owned reads as not-found / 404).
var ErrEndpointNotFound = errors.New("webhooks: endpoint not found")

// Config tunes the delivery worker.
type Config struct {
	MaxAttempts int // give up after this many failed attempts
	BatchLimit  int // max deliveries claimed per ProcessPending tick
}

// UseCase orchestrates webhook subscriptions and delivery.
type UseCase struct {
	store  Store
	client DeliveryClient
	obs    DeliveryObserver // optional; nil disables metrics
	cfg    Config
	logger *slog.Logger
}

// NewUseCase builds the webhooks usecase. A nil observer disables metrics; a
// nil logger falls back to the default slog logger.
func NewUseCase(store Store, client DeliveryClient, cfg Config, obs DeliveryObserver, logger ...*slog.Logger) *UseCase {
	l := slog.Default()
	if len(logger) > 0 && logger[0] != nil {
		l = logger[0]
	}
	return &UseCase{store: store, client: client, obs: obs, cfg: cfg, logger: l}
}

// CreateEndpoint validates the subscription through the domain constructor and
// persists it. Invalid URL / unknown event / weak secret surface as the domain
// errors (errors.Is-matchable) so the handler can map them to 400.
func (uc *UseCase) CreateEndpoint(ctx context.Context, userID uuid.UUID, rawURL string, events []string, secret string) (*domain.WebhookEndpoint, error) {
	parsed, err := parseEvents(events)
	if err != nil {
		return nil, err
	}
	ep, err := domain.NewWebhookEndpoint(userID, rawURL, parsed, secret)
	if err != nil {
		return nil, err
	}
	if err := uc.store.CreateEndpoint(ctx, ep); err != nil {
		return nil, fmt.Errorf("webhooks: create endpoint: %w", err)
	}
	return ep, nil
}

// parseEvents maps raw event strings to validated EventTypes, rejecting any
// unknown name (ErrUnknownEventType).
func parseEvents(events []string) ([]domain.EventType, error) {
	out := make([]domain.EventType, 0, len(events))
	for _, s := range events {
		et, err := domain.ParseEventType(s)
		if err != nil {
			return nil, err
		}
		out = append(out, et)
	}
	return out, nil
}

// ListEndpoints returns the caller's endpoints.
func (uc *UseCase) ListEndpoints(ctx context.Context, userID uuid.UUID) ([]*domain.WebhookEndpoint, error) {
	return uc.store.ListEndpoints(ctx, userID)
}

// DeleteEndpoint removes an endpoint after verifying the caller owns it.
func (uc *UseCase) DeleteEndpoint(ctx context.Context, userID, id uuid.UUID) error {
	if _, err := uc.ownedEndpoint(ctx, userID, id); err != nil {
		return err
	}
	return uc.store.DeleteEndpoint(ctx, id)
}

// TestEndpoint enqueues a synthetic ping delivery to the caller's endpoint,
// exercising the full delivery path (sign → guarded POST → mark) without
// waiting for a real domain event. Used by the "Test delivery" UI action.
func (uc *UseCase) TestEndpoint(ctx context.Context, userID, id uuid.UUID) error {
	ep, err := uc.ownedEndpoint(ctx, userID, id)
	if err != nil {
		return err
	}
	payload := buildEnvelope(domain.EventLeadCreated, json.RawMessage(`{"ping":true}`))
	return uc.enqueue(ctx, ep, domain.EventLeadCreated, payload)
}

// Publish fans an event out to every active endpoint of userID that subscribes
// to it, appending one delivery to the outbox per match. Returns the number
// enqueued. Wired to domain emit sites in Phase 2; unit-tested here.
func (uc *UseCase) Publish(ctx context.Context, userID uuid.UUID, event domain.EventType, data json.RawMessage) (int, error) {
	endpoints, err := uc.store.ListEndpoints(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("webhooks: list endpoints: %w", err)
	}
	payload := buildEnvelope(event, data)
	enqueued := 0
	for _, ep := range endpoints {
		if !ep.Active || !ep.Subscribes(event) {
			continue
		}
		if err := uc.enqueue(ctx, ep, event, payload); err != nil {
			uc.logger.ErrorContext(ctx, "webhooks: enqueue failed", "endpoint", ep.ID, "err", err)
			continue
		}
		enqueued++
	}
	return enqueued, nil
}

// enqueue builds a delivery for (endpoint, event, payload) and appends it.
func (uc *UseCase) enqueue(ctx context.Context, ep *domain.WebhookEndpoint, event domain.EventType, payload []byte) error {
	d, err := domain.NewDelivery(ep.UserID, ep.ID, event, payload)
	if err != nil {
		return err
	}
	return uc.store.EnqueueDelivery(ctx, d)
}

// ProcessPending claims a batch of due deliveries and attempts each. Returns the
// number successfully delivered. One delivery's failure never aborts the batch.
func (uc *UseCase) ProcessPending(ctx context.Context) (int, error) {
	due, err := uc.store.ClaimDueDeliveries(ctx, uc.cfg.BatchLimit, uc.cfg.MaxAttempts)
	if err != nil {
		return 0, fmt.Errorf("webhooks: claim due: %w", err)
	}
	delivered := 0
	for _, d := range due {
		if uc.deliverOne(ctx, d) {
			delivered++
		}
	}
	return delivered, nil
}

// deliverOne resolves the delivery's endpoint, signs the payload with its
// secret, and POSTs it through the guarded client. A deleted endpoint is a
// terminal failure (nothing to deliver to). Transport/non-2xx failures are
// retried until maxAttempts.
func (uc *UseCase) deliverOne(ctx context.Context, d *domain.WebhookDelivery) (ok bool) {
	ep, found, err := uc.store.GetEndpoint(ctx, d.EndpointID)
	if err != nil {
		uc.logger.ErrorContext(ctx, "webhooks: load endpoint", "endpoint", d.EndpointID, "err", err)
		return false
	}
	now := time.Now().UTC()
	if !found {
		// Endpoint deleted after enqueue: drop the delivery terminally.
		d.MarkFailed("endpoint deleted", 0, d.Attempts+1, now)
		uc.saveDelivery(ctx, d)
		uc.observe(d.EventType, false)
		return false
	}

	sig := domain.SignPayload(d.Payload, ep.Secret)
	status, err := uc.client.Deliver(ctx, ep.URL.String(), d.Payload, sig, d.EventID.String())
	if err != nil {
		d.MarkFailed(err.Error(), status, uc.cfg.MaxAttempts, now)
		uc.saveDelivery(ctx, d)
		uc.observe(d.EventType, false)
		return false
	}
	d.MarkDelivered(status, now)
	uc.saveDelivery(ctx, d)
	uc.observe(d.EventType, true)
	return true
}

func (uc *UseCase) saveDelivery(ctx context.Context, d *domain.WebhookDelivery) {
	if err := uc.store.SaveDelivery(ctx, d); err != nil {
		uc.logger.ErrorContext(ctx, "webhooks: save delivery", "delivery", d.ID, "err", err)
	}
}

func (uc *UseCase) observe(event domain.EventType, success bool) {
	if uc.obs != nil {
		uc.obs.OnWebhookDelivery(string(event), success)
	}
}

// ownedEndpoint loads an endpoint and verifies the caller owns it, collapsing
// "missing" and "not yours" into ErrEndpointNotFound.
func (uc *UseCase) ownedEndpoint(ctx context.Context, userID, id uuid.UUID) (*domain.WebhookEndpoint, error) {
	ep, found, err := uc.store.GetEndpoint(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("webhooks: get endpoint: %w", err)
	}
	if !found || ep.UserID != userID {
		return nil, ErrEndpointNotFound
	}
	return ep, nil
}

// envelope is the JSON body delivered to subscribers: a typed wrapper around the
// event-specific data, carrying the event name for routing.
type envelope struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// buildEnvelope marshals the delivery body. Marshaling a fixed-shape struct
// cannot fail in practice; on the impossible error we fall back to a minimal
// valid body so a delivery is never enqueued with an empty payload.
func buildEnvelope(event domain.EventType, data json.RawMessage) []byte {
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}
	b, err := json.Marshal(envelope{Event: string(event), Data: data})
	if err != nil {
		return fmt.Appendf(nil, `{"event":%q,"data":{}}`, string(event))
	}
	return b
}
