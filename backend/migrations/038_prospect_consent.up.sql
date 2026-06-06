-- Outbound-contact consent as a first-class compliance concept (152-ФЗ /
-- 38-ФЗ / GDPR-style). Every send decision is justified against this column
-- and the justification is auditable. See prospects/domain/consent.go for the
-- send-time rule (Prospect.AuthorizeOutbound) and ADR-002 for the model.
--
--   consent_status — none | obtained | withdrawn. New prospects default to
--                    'none' (cold): sends then require a logged lawful-basis
--                    override until consent is obtained. 'withdrawn' is the
--                    absolute red line — no override lifts it.
--   consent_source — where the basis came from (legacy / inbound_reply /
--                    import / manual / unsubscribe). Empty for 'none'.
--   consent_at     — when the basis was recorded. NULL for 'none' (no basis).
--
-- Mirrors the ENUM convention of prospect_status / verify_status (007/010).
CREATE TYPE consent_status AS ENUM ('none', 'obtained', 'withdrawn');

ALTER TABLE prospects
    ADD COLUMN consent_status consent_status NOT NULL DEFAULT 'none',
    ADD COLUMN consent_source VARCHAR(50)    NOT NULL DEFAULT '',
    ADD COLUMN consent_at     TIMESTAMPTZ;

-- Grandfather existing prospects to 'obtained'. They were collected under the
-- product's prior regime; silently leaving them at the new-default 'none'
-- would freeze live outreach (cold sends would all need an override). We
-- record the basis honestly rather than hiding it: source='legacy', at=now.
-- New rows inserted after this migration keep the 'none' default.
UPDATE prospects
    SET consent_status = 'obtained',
        consent_source = 'legacy',
        consent_at     = NOW()
    WHERE consent_status = 'none';
