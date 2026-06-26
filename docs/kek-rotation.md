# KEK rotation runbook (secrets at rest)

Floq encrypts client credentials at rest with AES-256-GCM under a single
key-encryption-key (KEK), `FLOQ_SECRETS_KEK` (base64-encoded 32 bytes). The
ciphertext carries **no key-id**, so rotation works by keeping the previous KEK
available as a decrypt-only fallback while every secret is re-encrypted under the
new key.

Encrypted columns: `user_settings.{telegram_bot_token, imap_password,
resend_api_key, smtp_password, ai_api_key}` and `onec_credentials.auth_secret`
(each as `<col>_enc` + `<col>_nonce`).

## When to rotate

- Suspected/confirmed KEK compromise.
- Scheduled key hygiene.
- Operator/key-custodian change.

## Procedure

1. **Generate a new KEK** (32 random bytes, base64):

   ```bash
   openssl rand -base64 32
   ```

2. **Deploy with both keys set** — new as primary, current as fallback:

   ```
   FLOQ_SECRETS_KEK=<new>          # primary: all writes + first decrypt attempt
   FLOQ_SECRETS_KEK_OLD=<old>      # fallback: decrypt-only, for not-yet-rotated rows
   ```

   The server boots normally. Live reads of old-key secrets succeed via the
   fallback; any **new** write is already sealed under the new key. A
   present-but-malformed `FLOQ_SECRETS_KEK_OLD` fails fast (never a silent
   fallback-disable).

3. **Re-encrypt every stored secret under the new key:**

   ```bash
   ./server -rotate-secrets
   ```

   Runs, logs counts, exits **without** migrating. It re-encrypts every
   non-empty secret (it cannot tell which key a row already uses, so the run is
   convergent — not a no-op — and safe to repeat). It aborts loudly if a secret
   decrypts under **neither** key (wrong `FLOQ_SECRETS_KEK_OLD`).

4. **Verify completeness (read-only gate):**

   ```bash
   ./server -verify-secrets-kek
   ```

   Decrypts every stored secret with the **primary key only**. Exit `0` and
   `needs-rotation=0` ⇒ all secrets are under the new key. Non-zero exit ⇒ some
   secret still needs the old key — **do not proceed**.

5. **Retire the old key.** Once step 4 is clean, redeploy with
   `FLOQ_SECRETS_KEK_OLD` **unset**. The old key can now be destroyed.

## Notes & failure modes

- **Order matters:** never remove `FLOQ_SECRETS_KEK_OLD` before `-verify-secrets-kek`
  reports `needs-rotation=0`. Removing it early makes any unrotated secret
  permanently undecryptable.
- **Single-instance / stop-then-start** for the step-2 deploy if you run a
  single binary; a rolling deploy is fine here because both old and new binaries
  understand the fallback (unlike the migration-047 drop, which was not
  rolling-safe).
- **KEK stability is an operational precondition** of the at-rest scheme
  generally (see migration `047_drop_plaintext_secrets`): losing both the
  primary and the fallback KEK means the ciphertext is unrecoverable. Back up
  keys in your secret manager before rotating.
- `-rotate-secrets` / `-verify-secrets-kek` mirror `-backfill-secrets`: one-shot,
  log, exit before migrations — safe to run against production out of band.
