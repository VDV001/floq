-- Drop the column (it depends on the type) before dropping the type itself.
ALTER TABLE prospects
    DROP COLUMN consent_status,
    DROP COLUMN consent_source,
    DROP COLUMN consent_at;

DROP TYPE consent_status;
