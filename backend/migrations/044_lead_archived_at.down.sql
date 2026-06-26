-- Restore the unfiltered qualification-distribution matview (migration 043
-- grain) FIRST, so the matview no longer references leads.archived_at, then
-- drop the column. Order matters: a matview referencing the column is a hard
-- dependency — dropping the column while it exists would error without CASCADE.
DROP MATERIALIZED VIEW IF EXISTS mv_analytics_qualification_distribution;
CREATE MATERIALIZED VIEW mv_analytics_qualification_distribution AS
SELECT
    l.user_id,
    date_trunc('day', q.generated_at)::date     AS day,
    (LEAST(GREATEST(q.score, 0), 99) / 10) * 10  AS bucket_lo,
    COUNT(*)                                     AS cnt
FROM qualifications q
JOIN leads l ON l.id = q.lead_id
GROUP BY l.user_id, date_trunc('day', q.generated_at)::date, (LEAST(GREATEST(q.score, 0), 99) / 10) * 10
WITH DATA;

CREATE UNIQUE INDEX mv_analytics_qual_dist_pk
    ON mv_analytics_qualification_distribution (user_id, day, bucket_lo);

ALTER TABLE leads DROP COLUMN IF EXISTS archived_at;
