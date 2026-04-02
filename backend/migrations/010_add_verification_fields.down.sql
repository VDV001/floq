ALTER TABLE prospects
    DROP COLUMN IF EXISTS phone,
    DROP COLUMN IF EXISTS telegram_username,
    DROP COLUMN IF EXISTS industry,
    DROP COLUMN IF EXISTS company_size,
    DROP COLUMN IF EXISTS context,
    DROP COLUMN IF EXISTS verify_status,
    DROP COLUMN IF EXISTS verify_score,
    DROP COLUMN IF EXISTS verify_details,
    DROP COLUMN IF EXISTS verified_at;

DROP TYPE IF EXISTS verify_status;
