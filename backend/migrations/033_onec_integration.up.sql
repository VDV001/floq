-- 1C integration: inbound/outbound sync ledger + per-user credentials.
--
-- onec_sync_records is the idempotency ledger. Every 1C event received via
-- webhook (and every action Floq pushes back, later) lands here exactly once.
-- The UNIQUE (user_id, external_id, external_type) constraint is the dedup
-- backbone: a replayed webhook hits the conflict and becomes a no-op instead
-- of double-applying. payload_hash lets a future reconciliation notice that
-- the same 1C object arrived with changed content.
--
-- Status/direction/kind are TEXT + CHECK (same convention as pending_replies
-- and audit_log) rather than pg ENUMs — cheaper to evolve as the event
-- vocabulary grows.
CREATE TABLE onec_sync_records (
    id            UUID PRIMARY KEY,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    external_id   TEXT NOT NULL,
    external_type TEXT NOT NULL,
    direction     TEXT NOT NULL,
    kind          TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'received',
    payload_hash  TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT onec_sync_direction_check CHECK (direction IN ('inbound', 'outbound')),
    -- Only 'received' for now (#106). 'processed'/'error' arrive with the
    -- mapping lifecycle in #107 via a follow-up migration — not pre-seeded
    -- here to avoid an unused-state CHECK.
    CONSTRAINT onec_sync_status_check    CHECK (status IN ('received')),
    CONSTRAINT onec_sync_kind_check      CHECK (kind IN ('payment', 'counterparty_created', 'order_status', 'shipment')),
    CONSTRAINT onec_sync_external_id_nonempty   CHECK (length(btrim(external_id)) > 0),
    CONSTRAINT onec_sync_external_type_nonempty CHECK (length(btrim(external_type)) > 0),
    CONSTRAINT onec_sync_dedup UNIQUE (user_id, external_id, external_type)
);

-- Primary read pattern: recent sync activity for a user, newest first.
CREATE INDEX idx_onec_sync_user_created ON onec_sync_records (user_id, created_at DESC);

-- Per-user 1C connection + secrets, isolated from user_settings so the
-- webhook secret and 1C credentials live apart from channel/AI config and
-- can be masked/rotated independently. One configuration per user.
--
-- webhook_secret MUST be a high-entropy random token (generated server-side
-- in #110, never a user-chosen string): it is the sole credential
-- authenticating inbound 1C webhooks, looked up directly in the index below.
CREATE TABLE onec_credentials (
    user_id        UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    base_url       TEXT NOT NULL DEFAULT '',
    auth_type      TEXT NOT NULL DEFAULT 'basic',
    auth_secret    TEXT NOT NULL DEFAULT '',
    webhook_secret TEXT NOT NULL DEFAULT '',
    is_active      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT onec_credentials_auth_type_check CHECK (auth_type IN ('basic', 'token'))
);

-- The webhook secret resolves the tenant on every inbound call, so it must be
-- globally unique (a collision would let one tenant's event hit another's
-- data) and indexed (avoid a full scan per webhook). Partial: the empty
-- default is "not configured" and must not collide across users.
CREATE UNIQUE INDEX idx_onec_credentials_webhook_secret
    ON onec_credentials (webhook_secret) WHERE webhook_secret <> '';
