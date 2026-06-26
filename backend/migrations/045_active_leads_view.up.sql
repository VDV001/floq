-- active_leads (#173 review follow-up): a single source of truth for "leads
-- the operator is actively working". archived_at IS NULL was being hand-copied
-- into ~13 lead-counting queries across 4 bounded contexts; the audit already
-- missed three of them (HITL pending queue, pending-reply stats, prospect-
-- suggestion counts). This view makes exclusion the DEFAULT: every feed/aggregate
-- query reads FROM active_leads instead of repeating the predicate, so a new
-- query is archive-correct by construction.
--
-- Lookups that legitimately need archived rows (GetLead, inbound dedup,
-- SetLeadArchived, ExportCSV backup) keep querying the base `leads` table.
--
-- NOTE: SELECT * freezes the column list at creation time — if a future
-- migration adds a leads column that a consumer needs through this view, the
-- view must be recreated (DROP + CREATE) to pick it up.
CREATE VIEW active_leads AS
    SELECT * FROM leads WHERE archived_at IS NULL;
