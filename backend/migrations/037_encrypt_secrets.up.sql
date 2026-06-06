-- At-rest encryption for client credentials (AES-256-GCM). This is step 1 of
-- a two-step migration so no secret is ever lost in a single irreversible
-- pass:
--
--   037 (this): add the *_enc / *_nonce byte columns next to the legacy
--               plaintext columns. A one-off backfill (server
--               -backfill-secrets) encrypts existing plaintext into them;
--               the application then writes only the enc columns and reads
--               enc-with-plaintext-fallback.
--   038 (later, after the backfill is verified): drop the plaintext columns.
--
-- bytea + nullable: a NULL enc column means "not yet encrypted", which the
-- read path treats as "fall back to the plaintext column". GCM stores the
-- ciphertext in *_enc and its per-secret random nonce in *_nonce.
--
-- onec_credentials.webhook_secret is intentionally NOT encrypted: it is a
-- server-generated lookup token resolved through a unique index on every
-- inbound webhook, not a client password. Encrypting it with a random nonce
-- would break that lookup.
ALTER TABLE user_settings
    ADD COLUMN telegram_bot_token_enc   bytea,
    ADD COLUMN telegram_bot_token_nonce bytea,
    ADD COLUMN imap_password_enc        bytea,
    ADD COLUMN imap_password_nonce      bytea,
    ADD COLUMN resend_api_key_enc       bytea,
    ADD COLUMN resend_api_key_nonce     bytea,
    ADD COLUMN smtp_password_enc        bytea,
    ADD COLUMN smtp_password_nonce      bytea,
    ADD COLUMN ai_api_key_enc           bytea,
    ADD COLUMN ai_api_key_nonce         bytea;

ALTER TABLE onec_credentials
    ADD COLUMN auth_secret_enc   bytea,
    ADD COLUMN auth_secret_nonce bytea;
