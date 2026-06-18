-- input_severity: the InputFirewall verdict for the inbound message that
-- triggered this auto-drafted reply (agent-security L2). The reply-dispatch
-- gate refuses to deliver a reply whose trigger was Block-flagged — a blocked
-- payload must not fan out into a customer-visible message even after human
-- approval. TEXT + CHECK (not a pg ENUM) mirrors channel/kind/status here, so
-- a new severity level needs no ALTER TYPE.
--
-- DEFAULT 'info' grandfathers existing rows to the safe baseline (the
-- pre-feature behaviour: no reply was ever gated), matching how a
-- severity-unset reply classifies in the domain.
ALTER TABLE pending_replies
    ADD COLUMN input_severity TEXT NOT NULL DEFAULT 'info'
        CONSTRAINT pending_replies_input_severity_check CHECK (input_severity IN ('info', 'warn', 'block'));
