-- Retention rollup for audit_log (#101). The per-call ledger (one row
-- per AI provider call) grows unbounded — ~200 bytes/row, 10k calls/day
-- is ~730 MB/year. A daily cron aggregates rows older than the retention
-- window into this table and deletes them from audit_log, preserving
-- cost trends (for the cost-summary report) while shedding the
-- high-cardinality per-call detail.
--
-- One row per (day, user_id, provider, model, request_type). Cost stays
-- in int64 micro-USD, summed — integer arithmetic, no float drift, same
-- convention as audit_log.cost_usd_micro.
--
-- user_id CASCADE mirrors audit_log: a deleted user's cost history goes
-- with them (GDPR erasure), no orphaned attribution rows.
--
-- No CHECK on request_type: this is a rollup of already-validated source
-- rows, and coupling it to the audit_log enum would force a constraint
-- bump here on every migration that extends that enum (see 029).
CREATE TABLE audit_log_daily (
    day                   DATE   NOT NULL,
    user_id               UUID   NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider              TEXT   NOT NULL,
    model                 TEXT   NOT NULL,
    request_type          TEXT   NOT NULL,
    total_calls           BIGINT NOT NULL CHECK (total_calls          >= 0),
    total_cost_usd_micro  BIGINT NOT NULL CHECK (total_cost_usd_micro >= 0),
    total_input_tokens    BIGINT NOT NULL CHECK (total_input_tokens   >= 0),
    total_output_tokens   BIGINT NOT NULL CHECK (total_output_tokens  >= 0),
    PRIMARY KEY (day, user_id, provider, model, request_type)
);

-- Read path mirrors audit_log's cost-summary access pattern: per-user
-- over a day range. The PK leads with `day`, so a user-scoped range scan
-- cannot use the PK prefix — it needs its own (user_id, day) index.
CREATE INDEX idx_audit_log_daily_user_day ON audit_log_daily (user_id, day);
