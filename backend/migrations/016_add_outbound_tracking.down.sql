ALTER TABLE outbound_messages
    DROP COLUMN IF EXISTS opened_at,
    DROP COLUMN IF EXISTS replied_at;
