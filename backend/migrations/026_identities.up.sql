-- Unified contact identity: one row per real person per user, regardless
-- of how many channels (email, phone, telegram) reach them. Phase 2
-- aggregation pivots cross-channel dedup on this table.
CREATE TABLE identities (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email             TEXT,
    phone             TEXT,
    telegram_username TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_identities_user_id ON identities(user_id);

-- Partial unique indexes enforce one identity per identifier per user
-- while still allowing NULLs (an identity can carry only some of the
-- three handles). Stored values are pre-normalized (lowercase + trim
-- for email/tg, digits-with-optional-plus for phone) — matching by
-- byte-exact equality is enough on the read path.
CREATE UNIQUE INDEX idx_identities_user_email ON identities(user_id, email) WHERE email IS NOT NULL;
CREATE UNIQUE INDEX idx_identities_user_phone ON identities(user_id, phone) WHERE phone IS NOT NULL;
CREATE UNIQUE INDEX idx_identities_user_tg    ON identities(user_id, telegram_username) WHERE telegram_username IS NOT NULL;

-- Link tables. Composite PKs cover the (lead_id, *) and (prospect_id, *)
-- lookups; secondary index on identity_id supports reverse fan-out
-- ("which leads point at this identity").
CREATE TABLE lead_identities (
    lead_id     UUID NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    identity_id UUID NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    linked_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (lead_id, identity_id)
);
CREATE INDEX idx_lead_identities_identity_id ON lead_identities(identity_id);

CREATE TABLE prospect_identities (
    prospect_id UUID NOT NULL REFERENCES prospects(id) ON DELETE CASCADE,
    identity_id UUID NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    linked_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (prospect_id, identity_id)
);
CREATE INDEX idx_prospect_identities_identity_id ON prospect_identities(identity_id);
