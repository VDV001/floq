-- Outbound 1C pushes (#108) record their result in the same ledger as inbound
-- capture. Migration 033 deliberately constrained status to 'received' only
-- (inbound capture) to avoid an unused-state CHECK; the outbound lifecycle now
-- needs the terminal states it foreshadowed: 'processed' (POST to 1C succeeded)
-- and 'error' (1C rejected or was unreachable). Drop and re-add the CHECK.
ALTER TABLE onec_sync_records DROP CONSTRAINT onec_sync_status_check;
ALTER TABLE onec_sync_records ADD CONSTRAINT onec_sync_status_check
    CHECK (status IN ('received', 'processed', 'error'));
