-- AI cost-tracking audit log. One row per AI provider call (Complete or
-- AnalyzeImage). Captured asynchronously by the RecordingProvider
-- decorator; population is best-effort and may drop on buffer overflow,
-- so this table is the *observed* spend ledger, not a financial source
-- of truth. Use it to answer "what did this user/lead cost us so far"
-- and "which model is burning budget today".
--
-- FK strategy:
--   * user_id CASCADE  — when a user is deleted (e.g. GDPR erasure)
--     their cost history must go with them; rows without an owner are
--     useless attribution-wise.
--   * lead_id / prospect_id SET NULL — cost rows survive entity removal
--     so per-user totals stay correct after a lead is deleted.
--
-- Cost is stored as int64 micro-USD (USD * 1_000_000). Integer
-- arithmetic avoids accumulated float error in aggregations and is
-- standard practice for monetary fields. Pricing lookup itself lives in
-- the Go layer (audit/pricing.go) keyed on (provider, model).
--
-- We intentionally do NOT persist the prompt or response content —
-- privacy hazard plus row-size blowup. If conversation-level forensics
-- is needed later, that belongs in a separate channel-scoped table.
CREATE TABLE audit_log (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    lead_id           UUID REFERENCES leads(id) ON DELETE SET NULL,
    prospect_id      UUID REFERENCES prospects(id) ON DELETE SET NULL,
    request_type      TEXT NOT NULL CHECK (request_type IN (
        'qualification',
        'draft_reply',
        'cold_message',
        'telegram_message',
        'telegram_reply',
        'call_brief',
        'followup',
        'image_analysis',
        'style_check'
    )),
    provider          TEXT NOT NULL,
    model             TEXT NOT NULL,
    input_tokens      INTEGER NOT NULL CHECK (input_tokens  >= 0),
    output_tokens     INTEGER NOT NULL CHECK (output_tokens >= 0),
    total_tokens      INTEGER NOT NULL CHECK (total_tokens  >= 0),
    cost_usd_micro    BIGINT  NOT NULL CHECK (cost_usd_micro >= 0),
    latency_ms        INTEGER NOT NULL CHECK (latency_ms     >= 0),
    status            TEXT    NOT NULL CHECK (status IN ('success', 'error')),
    error_message     TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT audit_log_error_message_only_on_error CHECK (
        (status = 'error' AND error_message IS NOT NULL)
        OR (status = 'success' AND error_message IS NULL)
    )
);

-- Primary access pattern: cost report for a user over a time range.
-- DESC matches "most recent first" UI ordering and `LIMIT N` paging.
CREATE INDEX idx_audit_log_user_created ON audit_log (user_id, created_at DESC);

-- Per-lead and per-prospect totals — used by the lead detail page
-- ("AI spent on this lead") and prospect drilldown. Partial indexes
-- skip the rows where the attribution column is NULL (qualification
-- without a lead yet, e.g. failed inbox normalization).
CREATE INDEX idx_audit_log_lead_created     ON audit_log (lead_id,     created_at DESC) WHERE lead_id     IS NOT NULL;
CREATE INDEX idx_audit_log_prospect_created ON audit_log (prospect_id, created_at DESC) WHERE prospect_id IS NOT NULL;
