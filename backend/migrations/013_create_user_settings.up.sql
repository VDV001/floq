CREATE TABLE user_settings (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    telegram_bot_token TEXT NOT NULL DEFAULT '',
    telegram_bot_active BOOLEAN NOT NULL DEFAULT FALSE,
    imap_host TEXT NOT NULL DEFAULT '',
    imap_port TEXT NOT NULL DEFAULT '993',
    imap_user TEXT NOT NULL DEFAULT '',
    imap_password TEXT NOT NULL DEFAULT '',
    resend_api_key TEXT NOT NULL DEFAULT '',
    ai_provider TEXT NOT NULL DEFAULT 'ollama',
    ai_model TEXT NOT NULL DEFAULT 'gemma3:4b',
    ai_api_key TEXT NOT NULL DEFAULT '',
    notify_telegram BOOLEAN NOT NULL DEFAULT TRUE,
    notify_email_digest BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
