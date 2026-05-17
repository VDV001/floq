-- Add 'chat_assist' to the audit_log.request_type CHECK list. The
-- chat assistant (internal/chat) bypasses every other use-case and so
-- was missing from the original enum in migration 028. Without this,
-- the recording provider would write rows with request_type='chat_assist'
-- and Postgres would reject them, leaving every chat call un-audited.
ALTER TABLE audit_log
    DROP CONSTRAINT audit_log_request_type_check;

ALTER TABLE audit_log
    ADD CONSTRAINT audit_log_request_type_check
    CHECK (request_type IN (
        'qualification',
        'draft_reply',
        'cold_message',
        'telegram_message',
        'telegram_reply',
        'call_brief',
        'followup',
        'image_analysis',
        'style_check',
        'chat_assist'
    ));
