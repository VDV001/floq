-- Capture operator attribution for HITL approve/reject decisions.
-- Nullable so existing rows (decided pre-migration) stay valid; new
-- transitions stamp it alongside decided_at via the domain factory.
-- ON DELETE SET NULL: if an operator account is later deleted, the
-- audit trail keeps decided_at but drops the dangling FK. The history
-- is preserved enough for "who decided this?" investigations while
-- still respecting tenant lifecycle.
ALTER TABLE pending_replies
ADD COLUMN decided_by UUID REFERENCES users(id) ON DELETE SET NULL;
