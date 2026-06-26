-- Add 'enrichment' to the audit_log.request_type CHECK list. The Phase-2
-- (#186) LLM company-enrichment extractor records its provider calls through
-- the RecordingProvider with request_type='enrichment'. Without this, Postgres
-- would reject those rows, leaving every enrichment call un-audited.
-- Must stay in sync with the RequestType enum in internal/audit/domain/entry.go.
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
        'chat_assist',
        'enrichment'
    ));
