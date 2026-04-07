CREATE TABLE IF NOT EXISTS prompt_feedback (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    original_body TEXT NOT NULL,
    edited_body TEXT NOT NULL,
    prospect_context TEXT NOT NULL DEFAULT '',
    channel VARCHAR(20) NOT NULL DEFAULT 'email',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_prompt_feedback_user ON prompt_feedback(user_id, created_at DESC);
