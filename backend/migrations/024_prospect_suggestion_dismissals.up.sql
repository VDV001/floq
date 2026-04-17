-- Tracks which prospect suggestions a user has dismissed for a given lead,
-- so the same cross-channel match isn't re-suggested after rejection.
CREATE TABLE prospect_suggestion_dismissals (
    lead_id UUID NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    prospect_id UUID NOT NULL REFERENCES prospects(id) ON DELETE CASCADE,
    dismissed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (lead_id, prospect_id)
);
-- No separate index on lead_id: the PK (lead_id, prospect_id) already covers
-- the NOT EXISTS subqueries in FindSuggestionsForLead and CountSuggestionsForUser
-- (btree prefix scan). An index on prospect_id is retained to support future
-- "undo all my dismissals for this prospect" workflows.
CREATE INDEX idx_prospect_suggestion_dismissals_prospect_id ON prospect_suggestion_dismissals(prospect_id);

-- Functional index for FindSuggestionsForLead / CountSuggestionsForUser: both
-- join prospects on LOWER(TRIM(p.name)) per user. Partial on status != 'converted'
-- because the matcher excludes converted prospects — keeps the index small.
-- LOWER + TRIM are IMMUTABLE under the default deterministic collation, so this
-- is indexable (unlike NOW() in partial indexes — see chronicles 2026-04-08).
CREATE INDEX idx_prospects_user_normalized_name
    ON prospects (user_id, LOWER(TRIM(name)))
    WHERE status <> 'converted';
