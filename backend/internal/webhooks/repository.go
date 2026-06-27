package webhooks

import (
	"context"
	"fmt"

	"github.com/daniil/floq/internal/webhooks/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the pgx-backed Store: webhook_endpoints + webhook_deliveries.
// The endpoint signing secret is encrypted at rest via the injected cipher.
type Repository struct {
	pool   *pgxpool.Pool
	cipher SecretCipher
}

// NewRepository constructs the webhooks repository with the at-rest cipher used
// to seal endpoint secrets.
func NewRepository(pool *pgxpool.Pool, cipher SecretCipher) *Repository {
	return &Repository{pool: pool, cipher: cipher}
}

var _ Store = (*Repository)(nil)

// CreateEndpoint inserts a new subscription, sealing the signing secret under
// the KEK before it ever touches the database.
func (r *Repository) CreateEndpoint(ctx context.Context, e *domain.WebhookEndpoint) error {
	ciphertext, nonce, err := r.cipher.Encrypt(e.Secret)
	if err != nil {
		return fmt.Errorf("webhooks: encrypt secret: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO webhook_endpoints (id, user_id, url, events, secret_ciphertext, secret_nonce, active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		e.ID, e.UserID, e.URL.String(), eventsToStrings(e.Events), ciphertext, nonce, e.Active)
	if err != nil {
		return fmt.Errorf("webhooks: create endpoint: %w", err)
	}
	return nil
}

// ListEndpoints returns a user's endpoints, newest first.
func (r *Repository) ListEndpoints(ctx context.Context, userID uuid.UUID) ([]*domain.WebhookEndpoint, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, url, events, secret_ciphertext, secret_nonce, active
		FROM webhook_endpoints
		WHERE user_id = $1
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("webhooks: list endpoints: %w", err)
	}
	defer rows.Close()

	var out []*domain.WebhookEndpoint
	for rows.Next() {
		e, err := r.scanEndpoint(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetEndpoint loads one endpoint by ID (ownership checked by the usecase).
func (r *Repository) GetEndpoint(ctx context.Context, id uuid.UUID) (*domain.WebhookEndpoint, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, url, events, secret_ciphertext, secret_nonce, active
		FROM webhook_endpoints WHERE id = $1`, id)
	e, err := r.scanEndpoint(row)
	if err == pgx.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return e, true, nil
}

// DeleteEndpoint removes an endpoint (and its deliveries, via ON DELETE CASCADE).
func (r *Repository) DeleteEndpoint(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM webhook_endpoints WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("webhooks: delete endpoint: %w", err)
	}
	return nil
}

// EnqueueDelivery appends a pending delivery to the outbox.
func (r *Repository) EnqueueDelivery(ctx context.Context, d *domain.WebhookDelivery) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO webhook_deliveries
			(id, event_id, user_id, endpoint_id, event_type, payload, status, attempts)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		d.ID, d.EventID, d.UserID, d.EndpointID, string(d.EventType), d.Payload,
		string(d.Status), d.Attempts)
	if err != nil {
		return fmt.Errorf("webhooks: enqueue delivery: %w", err)
	}
	return nil
}

// ClaimDueDeliveries returns up to limit pending deliveries that are due,
// earliest-due first, with attempts below maxAttempts. A delivery's effective
// due-time is COALESCE(next_retry_at, created_at): the domain-computed backoff
// schedule, or — when next_retry_at is null (never attempted) — its enqueue
// time. The backoff schedule is authored by domain.WebhookDelivery
// (NextRetryAfter); this query only compares the persisted due-time to now.
//
// This is equivalent to the older "next_retry_at IS NULL OR next_retry_at <=
// now()" predicate: created_at is always insert-time (DB DEFAULT now(), never set
// by EnqueueDelivery), so a null-next_retry_at row always satisfies created_at <=
// now() and is due immediately — exactly as before.
//
// Both the WHERE and ORDER BY key on COALESCE(next_retry_at, created_at) so the
// query rides idx_webhook_deliveries_due (migration 052): a forward index scan
// that stops at the first not-due row instead of scanning the whole pending
// partition. The id tiebreak makes the order total and stable.
//
// Phase 1 runs a single worker, so this plain SELECT needs no cross-instance
// lease; multi-instance exclusivity is a later hardening.
func (r *Repository) ClaimDueDeliveries(ctx context.Context, limit, maxAttempts int) ([]*domain.WebhookDelivery, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, event_id, user_id, endpoint_id, event_type, payload,
		       status, attempts, http_status, error, delivered_at, next_retry_at
		FROM webhook_deliveries
		WHERE status = 'pending' AND attempts < $2
		  AND COALESCE(next_retry_at, created_at) <= now()
		ORDER BY COALESCE(next_retry_at, created_at), id
		LIMIT $1`, limit, maxAttempts)
	if err != nil {
		return nil, fmt.Errorf("webhooks: claim due: %w", err)
	}
	defer rows.Close()

	var out []*domain.WebhookDelivery
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// SaveDelivery persists the outcome of a delivery attempt.
func (r *Repository) SaveDelivery(ctx context.Context, d *domain.WebhookDelivery) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE webhook_deliveries
		SET status = $2, attempts = $3, http_status = $4, error = $5,
		    delivered_at = $6, next_retry_at = $7, updated_at = now()
		WHERE id = $1`,
		d.ID, string(d.Status), d.Attempts, d.HTTPStatus, d.Error, d.DeliveredAt, d.NextRetryAt)
	if err != nil {
		return fmt.Errorf("webhooks: save delivery: %w", err)
	}
	return nil
}

// scanner is satisfied by both pgx.Row and pgx.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func (r *Repository) scanEndpoint(s scanner) (*domain.WebhookEndpoint, error) {
	var (
		e          domain.WebhookEndpoint
		rawURL     string
		events     []string
		ciphertext []byte
		nonce      []byte
	)
	if err := s.Scan(&e.ID, &e.UserID, &rawURL, &events, &ciphertext, &nonce, &e.Active); err != nil {
		return nil, err
	}
	secret, err := r.cipher.Decrypt(ciphertext, nonce)
	if err != nil {
		return nil, fmt.Errorf("webhooks: decrypt secret: %w", err)
	}
	e.URL = domain.WebhookURLFromStorage(rawURL)
	e.Events = stringsToEvents(events)
	e.Secret = secret
	return &e, nil
}

func scanDelivery(s scanner) (*domain.WebhookDelivery, error) {
	var (
		d         domain.WebhookDelivery
		eventType string
		status    string
	)
	if err := s.Scan(&d.ID, &d.EventID, &d.UserID, &d.EndpointID, &eventType, &d.Payload,
		&status, &d.Attempts, &d.HTTPStatus, &d.Error, &d.DeliveredAt, &d.NextRetryAt); err != nil {
		return nil, err
	}
	d.EventType = domain.EventType(eventType)
	d.Status = domain.DeliveryStatus(status)
	return &d, nil
}

func eventsToStrings(events []domain.EventType) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = string(e)
	}
	return out
}

func stringsToEvents(ss []string) []domain.EventType {
	out := make([]domain.EventType, len(ss))
	for i, s := range ss {
		out[i] = domain.EventType(s)
	}
	return out
}
