-- #198: index the delivery worker's claim scan on the effective due-time.
--
-- The worker claims due pending rows with
--   WHERE status='pending' AND COALESCE(next_retry_at, created_at) <= now()
--   ORDER BY COALESCE(next_retry_at, created_at)
-- (NULL next_retry_at = never attempted = due since creation).
--
-- The Phase 1 index idx_webhook_deliveries_pending was keyed on (updated_at).
-- It cannot serve this query well: it provides the wrong sort order, so with a
-- large backlog of not-yet-due (future next_retry_at) rows the planner scans the
-- whole pending partition and discards every not-due row by filter — measured at
-- ~90k rows / ~91k buffers read for a single 50-row claim. Worse, while that
-- index exists the planner still prefers it (its (updated_at) order matches the
-- old ORDER BY), so merely adding a due-keyed index alongside would leave the
-- new one unused.
--
-- Replace it with an expression index on the effective due-time. The claim query
-- both filters and orders by COALESCE(next_retry_at, created_at), so this becomes
-- a forward index scan that stops at the first not-due row (~0.04ms / ~50 buffers
-- for the same backlog), and is equally fast for a dense burst of due rows.
--
-- COALESCE(next_retry_at, created_at) is the effective due-time: the backoff
-- schedule, or — when next_retry_at is null (never attempted) — the enqueue time.
-- This equals the worker's old "next_retry_at IS NULL OR next_retry_at <= now()"
-- predicate because created_at is always insert-time (DEFAULT now(), never set by
-- EnqueueDelivery) and therefore always <= now() — so a null-next_retry_at row is
-- always due, exactly as before.
--
-- Locking: CREATE INDEX (non-CONCURRENTLY) takes a SHARE lock that blocks writes
-- to webhook_deliveries for the build. That is acceptable here: webhook delivery
-- ships dark behind WEBHOOKS_ENABLED (v0.63.0), so this table is effectively
-- empty when the migration runs and the build is sub-millisecond. The index
-- exists for FUTURE volume — by the time the table is large this migration is
-- long applied. (golang-migrate runs each file as one multi-statement Exec, i.e.
-- an implicit transaction, so CREATE INDEX CONCURRENTLY is not an option without
-- a separate no-transaction migration path; not warranted for an empty table.)
DROP INDEX idx_webhook_deliveries_pending;

CREATE INDEX idx_webhook_deliveries_due ON webhook_deliveries (COALESCE(next_retry_at, created_at))
    WHERE status = 'pending';
