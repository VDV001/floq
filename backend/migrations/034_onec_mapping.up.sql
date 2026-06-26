-- Per-user 1C→Floq mapping rules. Decouples the concrete 1C configuration
-- (which document types a given install emits) from Floq's canonical event
-- vocabulary: each user configures how their 1C's ExternalType strings map to
-- EventKind, plus where the counterparty email sits in the payload. Stored as
-- jsonb (a small, read-mostly array interpreted by onec/domain.MappingConfig);
-- one config per user, mirroring onec_credentials.
CREATE TABLE onec_mapping_configs (
    user_id    UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    rules      JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_active  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
