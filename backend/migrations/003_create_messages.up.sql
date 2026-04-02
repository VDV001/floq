CREATE TYPE message_direction AS ENUM ('inbound', 'outbound');

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lead_id UUID NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    direction message_direction NOT NULL,
    body TEXT NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_messages_lead_id ON messages(lead_id);
CREATE INDEX idx_messages_sent_at ON messages(sent_at);
