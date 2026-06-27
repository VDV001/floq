-- Revert to the Phase 1 (updated_at) partial index.
DROP INDEX idx_webhook_deliveries_due;

CREATE INDEX idx_webhook_deliveries_pending ON webhook_deliveries (updated_at)
    WHERE status = 'pending';
