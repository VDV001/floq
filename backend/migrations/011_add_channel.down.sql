ALTER TABLE outbound_messages DROP COLUMN IF EXISTS channel;
ALTER TABLE sequence_steps DROP COLUMN IF EXISTS channel;
DROP TYPE IF EXISTS step_channel;
