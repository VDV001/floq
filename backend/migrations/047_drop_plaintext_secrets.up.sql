-- Step 2 of the at-rest secrets migration (037 added the *_enc/*_nonce
-- columns + a backfill; this drops the legacy plaintext columns). By the time
-- this runs every secret must already live in its ciphertext column.
--
-- The server encrypts any straggler plaintext secrets at startup BEFORE
-- migrations run (cmd/server autoBackfillSecrets), so on a normally-booted
-- server this GUARD is a never-fire safety net.
--
-- GUARD: refuse to drop while any secret is still un-backfilled (plaintext
-- non-empty AND ciphertext NULL) — e.g. when migrations are applied out-of-band
-- via the `migrate` CLI without the server's pre-migration backfill. Dropping
-- then would silently destroy that secret.
--
-- RECOVERY if this RAISE fires (CLI path): golang-migrate marks the schema
-- dirty at v47, so first clear it with `migrate force 46`, then start the
-- server once (its pre-migration autoBackfillSecrets encrypts the stragglers)
-- to re-apply 047 cleanly. The guard checks ciphertext PRESENCE, not
-- decryptability — KEK stability across this deploy is an operational
-- precondition (a secret encrypted under a lost KEK is unrecoverable).
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM user_settings WHERE
            (telegram_bot_token <> '' AND telegram_bot_token_enc IS NULL) OR
            (imap_password       <> '' AND imap_password_enc       IS NULL) OR
            (resend_api_key      <> '' AND resend_api_key_enc      IS NULL) OR
            (smtp_password       <> '' AND smtp_password_enc       IS NULL) OR
            (ai_api_key          <> '' AND ai_api_key_enc          IS NULL)
    ) THEN
        RAISE EXCEPTION 'refusing to drop plaintext secrets: un-backfilled user_settings rows exist (plaintext set, *_enc NULL). Run the secret backfill on the prior release first.';
    END IF;

    IF EXISTS (
        SELECT 1 FROM onec_credentials
        WHERE auth_secret <> '' AND auth_secret_enc IS NULL
    ) THEN
        RAISE EXCEPTION 'refusing to drop plaintext secrets: un-backfilled onec_credentials rows exist (auth_secret set, auth_secret_enc NULL). Run the secret backfill on the prior release first.';
    END IF;
END $$;

ALTER TABLE user_settings
    DROP COLUMN telegram_bot_token,
    DROP COLUMN imap_password,
    DROP COLUMN resend_api_key,
    DROP COLUMN smtp_password,
    DROP COLUMN ai_api_key;

ALTER TABLE onec_credentials
    DROP COLUMN auth_secret;
