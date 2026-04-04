ALTER TABLE user_settings
    DROP COLUMN IF EXISTS auto_qualify,
    DROP COLUMN IF EXISTS auto_draft,
    DROP COLUMN IF EXISTS auto_send,
    DROP COLUMN IF EXISTS auto_send_delay_min,
    DROP COLUMN IF EXISTS auto_followup,
    DROP COLUMN IF EXISTS auto_followup_days,
    DROP COLUMN IF EXISTS auto_prospect_to_lead,
    DROP COLUMN IF EXISTS auto_verify_import;
