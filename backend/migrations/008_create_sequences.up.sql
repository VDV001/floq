CREATE TABLE sequences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE sequence_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sequence_id UUID NOT NULL REFERENCES sequences(id) ON DELETE CASCADE,
    step_order INT NOT NULL,
    delay_days INT NOT NULL DEFAULT 0,
    prompt_hint TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(sequence_id, step_order)
);

CREATE INDEX idx_sequences_user_id ON sequences(user_id);
CREATE INDEX idx_sequence_steps_sequence_id ON sequence_steps(sequence_id);
