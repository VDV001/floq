-- #206 Part C: durable, retryable auto-qualification queue.
--
-- Auto-qualification (AI scoring of an inbound lead) used to run in a
-- fire-and-forget goroutine: if it failed or the process died, the lead was
-- never qualified and the lead.qualified webhook was lost (at-most-once). This
-- table is the table-as-queue that replaces the goroutine. The inbox poller
-- enqueues one job per inbound message (atomically with the lead on first
-- contact); a worker claims due jobs, runs the AI qualifier, and commits the
-- qualification + lead status + lead.qualified webhook enqueue in one
-- transaction (fail-closed). It mirrors webhook_deliveries: a pending/done/failed
-- lifecycle with an exponential-backoff next_retry_at and a dead-letter cap.

CREATE TABLE lead_qualification_jobs (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    lead_id       uuid NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- The AI qualifier inputs are captured at enqueue time so a retry re-runs the
    -- exact qualification (e.g. the email body plus any extracted attachment text,
    -- which is ephemeral and not stored on the lead).
    contact_name  text NOT NULL,
    channel       varchar(20) NOT NULL,
    qualify_text  text NOT NULL,
    status        varchar(20) NOT NULL DEFAULT 'pending',
    attempts      int NOT NULL DEFAULT 0,
    last_error    text NOT NULL DEFAULT '',
    -- next_retry_at is the domain-authored backoff schedule (NULL = due now). The
    -- worker claims pending rows where it is NULL or in the past.
    next_retry_at timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- Index the worker's claim scan on the effective due-time, exactly as
-- webhook_deliveries (migration 052): the claim both filters and orders by
-- COALESCE(next_retry_at, created_at), so this is a forward index scan that stops
-- at the first not-due row instead of scanning the whole pending partition.
-- created_at is always insert-time (DEFAULT now(), never set by the enqueuer), so
-- a null-next_retry_at row is always due — equivalent to "next_retry_at IS NULL
-- OR next_retry_at <= now()".
CREATE INDEX idx_lead_qualification_jobs_due
    ON lead_qualification_jobs (COALESCE(next_retry_at, created_at))
    WHERE status = 'pending';
