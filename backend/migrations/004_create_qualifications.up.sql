CREATE TABLE qualifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lead_id UUID UNIQUE NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    identified_need TEXT NOT NULL DEFAULT '',
    estimated_budget VARCHAR(255) NOT NULL DEFAULT '',
    deadline VARCHAR(255) NOT NULL DEFAULT '',
    score INT NOT NULL DEFAULT 0,
    score_reason TEXT NOT NULL DEFAULT '',
    recommended_action TEXT NOT NULL DEFAULT '',
    provider_used VARCHAR(50) NOT NULL DEFAULT '',
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
