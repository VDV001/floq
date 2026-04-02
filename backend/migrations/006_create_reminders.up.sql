CREATE TABLE reminders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lead_id UUID NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    message TEXT NOT NULL,
    snoozed_until TIMESTAMPTZ,
    dismissed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reminders_lead_id ON reminders(lead_id);
CREATE INDEX idx_reminders_dismissed ON reminders(dismissed) WHERE NOT dismissed;
