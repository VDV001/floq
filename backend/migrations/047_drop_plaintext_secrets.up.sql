-- Step 2 of the at-rest secrets migration (037 added the *_enc/*_nonce
-- columns + a one-off backfill; this drops the legacy plaintext columns). By
-- the time this runs every secret must already live in its ciphertext column.
--
-- GUARD: refuse to drop while any secret is still un-backfilled (plaintext
-- non-empty AND ciphertext NULL). Dropping then would silently destroy that
-- secret. The backfill (`server -backfill-secrets`) shipped with the 037
-- release and must have been run on a prior version — note that migrations run
-- at server startup BEFORE the backfill step, so the backfill cannot be run on
-- a release that already contains this migration. If this RAISE fires, deploy
-- the prior release, run the backfill, then re-apply.
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
