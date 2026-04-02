CREATE TYPE verify_status AS ENUM ('not_checked', 'valid', 'risky', 'invalid');

ALTER TABLE prospects
    ADD COLUMN phone VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN telegram_username VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN industry VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN company_size VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN context TEXT NOT NULL DEFAULT '',
    ADD COLUMN verify_status verify_status NOT NULL DEFAULT 'not_checked',
    ADD COLUMN verify_score INT NOT NULL DEFAULT 0,
    ADD COLUMN verify_details JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN verified_at TIMESTAMPTZ;

CREATE INDEX idx_prospects_verify_status ON prospects(verify_status);
CREATE INDEX idx_prospects_telegram_username ON prospects(telegram_username) WHERE telegram_username != '';
