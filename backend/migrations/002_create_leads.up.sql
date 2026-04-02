CREATE TYPE lead_channel AS ENUM ('telegram', 'email');
CREATE TYPE lead_status AS ENUM ('new', 'qualified', 'in_conversation', 'followup', 'closed');

CREATE TABLE leads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel lead_channel NOT NULL,
    contact_name VARCHAR(255) NOT NULL DEFAULT '',
    company VARCHAR(255) NOT NULL DEFAULT '',
    first_message TEXT NOT NULL DEFAULT '',
    status lead_status NOT NULL DEFAULT 'new',
    telegram_chat_id BIGINT,
    email_address VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_leads_user_id ON leads(user_id);
CREATE INDEX idx_leads_status ON leads(status);
