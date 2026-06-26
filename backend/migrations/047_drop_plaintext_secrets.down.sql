-- Reverse of 047: re-create the legacy plaintext secret columns with their
-- original types/defaults so the schema rolls back cleanly. This restores the
-- SCHEMA but NOT the DATA — the plaintext values are gone; secrets now live
-- only in the *_enc/*_nonce columns. After a rollback the read path falls back
-- to these (now-empty) plaintext columns only for rows with NULL ciphertext,
-- so backfilled secrets keep working; the columns come back empty.
ALTER TABLE user_settings
    ADD COLUMN telegram_bot_token TEXT         NOT NULL DEFAULT '',
    ADD COLUMN imap_password      TEXT         NOT NULL DEFAULT '',
    ADD COLUMN resend_api_key     TEXT         NOT NULL DEFAULT '',
    ADD COLUMN smtp_password      VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN ai_api_key         TEXT         NOT NULL DEFAULT '';

ALTER TABLE onec_credentials
    ADD COLUMN auth_secret TEXT NOT NULL DEFAULT '';
