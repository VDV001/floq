-- Human-in-the-loop approval queue for auto-drafted inbox replies.
-- Each row is a customer-visible message that the inbox flow generated
-- automatically (e.g. a booking link triggered by DetectCallAgreement
-- in the Telegram bot) and parked here until an operator decides to
-- approve and send it, or to reject it outright. Status transitions
-- mirror the inbox.PendingReply aggregate state machine.
CREATE TABLE pending_replies (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    lead_id    UUID NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    channel    TEXT NOT NULL,
    kind       TEXT NOT NULL,
    body       TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    decided_at TIMESTAMPTZ,
    sent_at    TIMESTAMPTZ,
    CONSTRAINT pending_replies_channel_check  CHECK (channel IN ('telegram', 'email')),
    CONSTRAINT pending_replies_kind_check     CHECK (kind IN ('booking_link')),
    CONSTRAINT pending_replies_status_check   CHECK (status IN ('pending', 'approved', 'sent', 'rejected')),
    CONSTRAINT pending_replies_body_nonempty  CHECK (length(btrim(body)) > 0)
);

-- Operator inbox queue lookup: "everything pending for this user, newest first".
CREATE INDEX idx_pending_replies_user_status ON pending_replies(user_id, status, created_at DESC);

-- Per-lead drill-down: "show pending replies tied to this lead".
CREATE INDEX idx_pending_replies_user_lead ON pending_replies(user_id, lead_id, created_at DESC);
