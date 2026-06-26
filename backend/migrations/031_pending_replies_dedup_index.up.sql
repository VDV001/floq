-- Partial unique index that collapses simultaneously-pending drafts
-- with identical content. Telegram long-poll reconnects can re-fire
-- DetectCallAgreement on the same inbound message twice, which would
-- otherwise enqueue duplicate approval buttons for the operator and
-- risk a double-send if both get approved.
--
-- Scoped to status='pending' so completed/decided rows do NOT block a
-- fresh proposal for the same content later (e.g. operator rejected
-- the first draft; we want a follow-up Propose to be enqueueable).
--
-- md5(body) keeps the index key bounded — body may be hundreds of
-- bytes (booking link + accompanying message). md5 is IMMUTABLE so
-- the expression is index-safe.
CREATE UNIQUE INDEX idx_pending_replies_dedup_pending
ON pending_replies (user_id, lead_id, kind, md5(body))
WHERE status = 'pending';
