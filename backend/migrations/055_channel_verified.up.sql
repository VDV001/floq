-- #222: honest onboarding "Готово" — a channel is done only after a
-- successful connection test, not merely because its fields are filled.
--
-- Prior to this, AIActive/SMTPActive/IMAPActive were computed from
-- "credentials present", so Ollama showed "готово" with no reachability
-- check at all, and a wrong cloud key still read as done. These flags
-- persist the result of the user's connection test: set true when a test
-- passes, cleared when the channel's credentials change (must re-verify).
--
-- Default false: existing rows are treated as unverified until the user
-- re-runs "Проверить подключение" once. We deliberately do NOT backfill
-- true from "creds present" — that would re-introduce the very lie this
-- fixes.
ALTER TABLE user_settings ADD COLUMN ai_verified   boolean NOT NULL DEFAULT false;
ALTER TABLE user_settings ADD COLUMN smtp_verified boolean NOT NULL DEFAULT false;
ALTER TABLE user_settings ADD COLUMN imap_verified boolean NOT NULL DEFAULT false;
