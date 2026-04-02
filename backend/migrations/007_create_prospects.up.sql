CREATE TYPE prospect_source AS ENUM ('manual', 'csv');
CREATE TYPE prospect_status AS ENUM ('new', 'in_sequence', 'replied', 'converted', 'opted_out');

CREATE TABLE prospects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    company VARCHAR(255) NOT NULL DEFAULT '',
    title VARCHAR(255) NOT NULL DEFAULT '',
    email VARCHAR(255) NOT NULL,
    source prospect_source NOT NULL DEFAULT 'manual',
    status prospect_status NOT NULL DEFAULT 'new',
    converted_lead_id UUID REFERENCES leads(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_prospects_user_id ON prospects(user_id);
CREATE INDEX idx_prospects_status ON prospects(status);
CREATE INDEX idx_prospects_email ON prospects(email);
