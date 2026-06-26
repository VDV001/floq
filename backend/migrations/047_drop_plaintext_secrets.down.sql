-- Reverse of 047: re-create the legacy plaintext secret columns with their
-- original types/defaults so the schema rolls back cleanly. This restores the
-- SCHEMA but NOT the DATA — the plaintext values are gone; secrets live only in
-- the *_enc/*_nonce columns and the current binary reads them only from there.
-- The re-added columns come back EMPTY and are NOT read by the current binary
-- (its SELECTs no longer reference them). They exist only so a prior release —
-- which still reads plaintext with a ciphertext-preferring fallback — keeps
-- working after a FULL rollback (binary + schema); backfilled secrets survive
-- because that fallback prefers ciphertext, not because of these empty columns.
ALTER TABLE user_settings
    ADD COLUMN telegram_bot_token TEXT         NOT NULL DEFAULT '',
    ADD COLUMN imap_password      TEXT         NOT NULL DEFAULT '',
    ADD COLUMN resend_api_key     TEXT         NOT NULL DEFAULT '',
    ADD COLUMN smtp_password      VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN ai_api_key         TEXT         NOT NULL DEFAULT '';

ALTER TABLE onec_credentials
    ADD COLUMN auth_secret TEXT NOT NULL DEFAULT '';
