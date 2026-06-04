-- Revert to the inbound-only status CHECK. Fails if any outbound rows in a
-- 'processed'/'error' state remain — clear them before rolling back.
ALTER TABLE onec_sync_records DROP CONSTRAINT onec_sync_status_check;
ALTER TABLE onec_sync_records ADD CONSTRAINT onec_sync_status_check
    CHECK (status IN ('received'));
