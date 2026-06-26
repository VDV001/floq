-- Auto-enrichment (#182): per-user, per-domain company data scraped from the
-- company's public website. The table doubles as the work queue — a row is
-- inserted in 'pending' on lead/prospect create and the enrichment cron worker
-- claims due rows (pending, or enriched-and-expired) to (re)scrape.
CREATE TABLE company_enrichment (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    domain      varchar(255) NOT NULL,
    status      varchar(20) NOT NULL DEFAULT 'pending',
    profile     jsonb NOT NULL DEFAULT '{}',
    error       text NOT NULL DEFAULT '',
    attempts    int NOT NULL DEFAULT 0,
    enriched_at timestamptz,
    expires_at  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, domain)
);

-- Partial index for the worker's claim scan: it only ever looks at pending rows
-- (expired-enriched rows are found via expires_at, see the worker query).
CREATE INDEX idx_company_enrichment_pending ON company_enrichment (updated_at)
    WHERE status = 'pending';
