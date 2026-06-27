package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// DeliveryStatus is the lifecycle state of one delivery attempt-set.
type DeliveryStatus string

const (
	// DeliveryPending is queued or mid-retry: the worker will (re)attempt it.
	DeliveryPending DeliveryStatus = "pending"
	// DeliverySucceeded got a 2xx; terminal.
	DeliverySucceeded DeliveryStatus = "succeeded"
	// DeliveryFailed exhausted its retries; terminal (dead-letter).
	DeliveryFailed DeliveryStatus = "failed"
)

// ErrEmptyPayload guards against enqueuing a delivery with no body.
var ErrEmptyPayload = errors.New("webhooks: delivery payload is empty")

// RetryBaseBackoff is the first retry delay; it doubles per attempt. Mirrors the
// outbound Resend retry (200ms base, ×2) but on the worker timescale. Exported
// so the repository's SQL due-ness predicate stays in sync with NextRetryAfter.
const RetryBaseBackoff = 30 * time.Second

// WebhookDelivery is the outbox record for delivering one event to one endpoint.
// It is the unit of work for the delivery worker and the at-least-once dedup key
// (EventID) for receivers.
type WebhookDelivery struct {
	ID          uuid.UUID
	EventID     uuid.UUID // idempotency key sent as X-Floq-Event-Id; stable across retries of this delivery so receivers dedup
	UserID      uuid.UUID
	EndpointID  uuid.UUID
	EventType   EventType
	Payload     []byte
	Status      DeliveryStatus
	Attempts    int
	HTTPStatus  int
	Error       string
	DeliveredAt *time.Time
	// NextRetryAt is when this delivery is next eligible for a retry (nil = due
	// now, i.e. never attempted or terminal). The worker claims rows where it is
	// null or in the past; this is the domain-authoritative backoff schedule.
	NextRetryAt *time.Time
}

// NewDelivery builds a pending delivery for (endpoint, event). It generates both
// a row ID and an EventID — the latter is sent as the X-Floq-Event-Id header so
// a receiver can dedup retries of this delivery.
func NewDelivery(userID, endpointID uuid.UUID, eventType EventType, payload []byte) (*WebhookDelivery, error) {
	if len(payload) == 0 {
		return nil, ErrEmptyPayload
	}
	return &WebhookDelivery{
		ID:         uuid.New(),
		EventID:    uuid.New(),
		UserID:     userID,
		EndpointID: endpointID,
		EventType:  eventType,
		Payload:    payload,
		Status:     DeliveryPending,
		Attempts:   0,
	}, nil
}

// MarkDelivered records a successful 2xx delivery (terminal). now is passed in
// (not time.Now) so the transition is deterministic and testable.
func (d *WebhookDelivery) MarkDelivered(httpStatus int, now time.Time) {
	d.Attempts++
	d.Status = DeliverySucceeded
	d.HTTPStatus = httpStatus
	d.Error = ""
	t := now.UTC()
	d.DeliveredAt = &t
	d.NextRetryAt = nil
}

// MarkFailed records a failed attempt. The delivery stays pending (retryable)
// until attempts reach maxAttempts, at which point it becomes terminally failed
// (dead-letter). httpStatus is 0 for transport-level failures (dial/timeout).
// While retryable, NextRetryAt is set to the exponential-backoff schedule off
// now (domain-authoritative); on terminal failure it is cleared.
func (d *WebhookDelivery) MarkFailed(reason string, httpStatus, maxAttempts int, now time.Time) {
	d.Attempts++
	d.HTTPStatus = httpStatus
	d.Error = reason
	if d.Attempts >= maxAttempts {
		d.Status = DeliveryFailed
		d.NextRetryAt = nil
	} else {
		d.Status = DeliveryPending
		next := d.NextRetryAfter(now)
		d.NextRetryAt = &next
	}
}

// NextRetryAfter returns when this delivery is next eligible, computed as an
// exponential backoff from base by the current attempt count. base is passed in
// (not time.Now) so it is deterministic and testable.
func (d *WebhookDelivery) NextRetryAfter(base time.Time) time.Time {
	backoff := RetryBaseBackoff
	for i := 1; i < d.Attempts; i++ {
		backoff *= 2
	}
	return base.Add(backoff)
}
