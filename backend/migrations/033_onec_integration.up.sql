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
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    external_id   TEXT NOT NULL,
    external_type TEXT NOT NULL,
    direction     TEXT NOT NULL,
    kind          TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'received',
    payload_hash  TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT onec_sync_direction_check CHECK (direction IN ('inbound', 'outbound')),
    CONSTRAINT onec_sync_status_check    CHECK (status IN ('received', 'processed', 'error')),
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
