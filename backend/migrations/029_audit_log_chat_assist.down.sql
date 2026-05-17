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
        'style_check'
    ));
