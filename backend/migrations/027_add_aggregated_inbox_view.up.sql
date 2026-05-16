-- Phase 2 multi-source aggregation (#27 PR3) user preference: when true,
-- the lead detail page shows messages from every lead sharing the same
-- Identity. Default TRUE — the aggregated view is the recommended UX
-- once the IdentityResolver pipeline (PR2) has run a backfill.
--
-- The dual default (here + Go-side struct default in repository.go) is
-- intentional: an ErrNoRows lookup builds a Settings struct without
-- touching the DB, and the struct must agree with the column default
-- so a user with no `user_settings` row sees the same value as one
-- with a freshly-inserted row.
ALTER TABLE user_settings
    ADD COLUMN IF NOT EXISTS aggregated_inbox_view BOOLEAN NOT NULL DEFAULT TRUE;
