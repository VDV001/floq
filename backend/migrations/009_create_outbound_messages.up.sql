CREATE TYPE outbound_status AS ENUM ('draft', 'approved', 'sent', 'rejected');

CREATE TABLE outbound_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    prospect_id UUID NOT NULL REFERENCES prospects(id) ON DELETE CASCADE,
    sequence_id UUID NOT NULL REFERENCES sequences(id) ON DELETE CASCADE,
    step_order INT NOT NULL,
    body TEXT NOT NULL,
    status outbound_status NOT NULL DEFAULT 'draft',
    scheduled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_outbound_messages_prospect_id ON outbound_messages(prospect_id);
CREATE INDEX idx_outbound_messages_status ON outbound_messages(status);
CREATE INDEX idx_outbound_messages_scheduled ON outbound_messages(scheduled_at) WHERE status = 'approved';
