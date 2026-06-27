-- Outgoing webhooks (#181): per-user webhook subscriptions (endpoints) and the
-- delivery outbox. Endpoints name a public URL, a set of subscribed events, and
-- a signing secret. Deliveries are the at-least-once outbox: one row per
-- (event, endpoint), claimed by the delivery worker, signed and POSTed with
-- retries.

CREATE TABLE webhook_endpoints (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    url        text NOT NULL,
    events     text[] NOT NULL,
    secret     text NOT NULL,
    active     boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_webhook_endpoints_user ON webhook_endpoints (user_id, created_at DESC);

CREATE TABLE webhook_deliveries (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id     uuid NOT NULL,
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint_id  uuid NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    event_type   varchar(50) NOT NULL,
    payload      jsonb NOT NULL,
    status       varchar(20) NOT NULL DEFAULT 'pending',
    attempts     int NOT NULL DEFAULT 0,
    http_status  int NOT NULL DEFAULT 0,
    error        text NOT NULL DEFAULT '',
    delivered_at timestamptz,
    -- next_retry_at is the domain-authored backoff schedule (NULL = due now).
    -- The worker claims pending rows where it is NULL or in the past.
    next_retry_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- Partial index for the worker's claim scan over due pending rows.
CREATE INDEX idx_webhook_deliveries_pending ON webhook_deliveries (updated_at)
    WHERE status = 'pending';
