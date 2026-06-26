-- Index supporting the archive view (#174). ListArchivedLeads filters
-- `archived_at IS NOT NULL` and orders `archived_at DESC` scoped per user; with
-- no index that is a full per-user scan + in-memory sort on every request,
-- whose cost grows with total lead volume. A PARTIAL index keyed on
-- (user_id, archived_at DESC) WHERE archived_at IS NOT NULL serves both the
-- predicate and the ordering while staying tiny — it only holds archived rows,
-- the small minority. The IS NOT NULL predicate is IMMUTABLE (unlike NOW()), so
-- it is legal in a partial-index WHERE — see chronicles 2026-04-08.
CREATE INDEX idx_leads_archived_at
    ON leads (user_id, archived_at DESC)
    WHERE archived_at IS NOT NULL;
