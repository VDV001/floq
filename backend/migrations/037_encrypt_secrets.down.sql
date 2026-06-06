-- Reverse of 037: drop the encrypted-secret columns. The legacy plaintext
-- columns are untouched (they still hold the secrets while 037 is in force),
-- so this is non-destructive.
ALTER TABLE onec_credentials
    DROP COLUMN auth_secret_enc,
    DROP COLUMN auth_secret_nonce;

ALTER TABLE user_settings
    DROP COLUMN telegram_bot_token_enc,
    DROP COLUMN telegram_bot_token_nonce,
    DROP COLUMN imap_password_enc,
    DROP COLUMN imap_password_nonce,
    DROP COLUMN resend_api_key_enc,
    DROP COLUMN resend_api_key_nonce,
    DROP COLUMN smtp_password_enc,
    DROP COLUMN smtp_password_nonce,
    DROP COLUMN ai_api_key_enc,
    DROP COLUMN ai_api_key_nonce;
