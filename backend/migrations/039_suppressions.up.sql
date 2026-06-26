-- suppressions: addresses that must never be contacted again on a channel.
-- The hard pre-check ahead of the consent rule (see prospects/domain/
-- suppression.go and ADR-002). Populated by unsubscribe (and later bounce
-- handling). The send-gate consults this BEFORE Prospect.AuthorizeOutbound.
--
-- channel is TEXT + CHECK rather than a pg ENUM so new channels (whatsapp,
-- sms) can be added without an ALTER TYPE — same convention as onec /
-- pending_replies. The address is stored already-normalized by the domain
-- (NormalizeSuppressionAddress), so UNIQUE dedupes case-insensitively without
-- a functional index, and that same UNIQUE index serves the lookup.
CREATE TABLE suppressions (
    id         UUID PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel    TEXT NOT NULL CHECK (channel IN ('email', 'telegram')),
    address    VARCHAR(255) NOT NULL,
    reason     VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, channel, address)
);
