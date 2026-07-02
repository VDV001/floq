-- #241 (follow-up #222): honest onboarding «Готово» for the outbound-email
-- step. That step is done when smtp_active OR resend_active; #222 made
-- SMTP honest (verified) but left resend_active = "key present", so an
-- unvalidated Resend key still read as done. resend_verified persists the
-- result of the Resend connection test, matching ai/smtp/imap_verified.
--
-- Default false, no backfill — same rationale as 055: backfilling from
-- "key present" would re-introduce the lie this fixes.
ALTER TABLE user_settings ADD COLUMN resend_verified boolean NOT NULL DEFAULT false;
