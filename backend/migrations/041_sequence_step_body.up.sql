-- Manual email/message body for a sequence step. When non-empty, the launch
-- uses this text verbatim instead of generating with AI — so operators can
-- write a step by hand without an AI provider configured.
ALTER TABLE sequence_steps ADD COLUMN body TEXT NOT NULL DEFAULT '';
