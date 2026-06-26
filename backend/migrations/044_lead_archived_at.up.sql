-- Real lead archive (#173). archived_at is ORTHOGONAL to the pipeline status:
-- a non-null value hides the lead from working feeds (inbox list, stale-lead
-- reminders) and from analytics aggregates, WITHOUT changing its status. Earlier
-- attempts modelled archive as status='closed', but closed is a terminal,
-- analytics-significant business state ("closed deal") — conflating the two
-- distorted the funnel. Nullable, defaults NULL, so every existing lead is
-- grandfathered as active.
ALTER TABLE leads ADD COLUMN archived_at timestamptz;

-- Rebuild the qualification-distribution matview to exclude archived leads, so
-- the period-windowed funnel (migration 043) matches the base-table inbox
-- analytics — both now filter archived_at IS NULL. Grain, GROUP BY and the
-- REFRESH CONCURRENTLY unique index are unchanged from 043; only the WHERE is
-- added. Re-materialised WITH DATA; the next RefreshCron pass restores the live
-- snapshot. (mv_analytics_sequence_step_conversion does not touch leads, so it
-- is left untouched.)
DROP MATERIALIZED VIEW IF EXISTS mv_analytics_qualification_distribution;
CREATE MATERIALIZED VIEW mv_analytics_qualification_distribution AS
SELECT
    l.user_id,
    date_trunc('day', q.generated_at)::date     AS day,
    (LEAST(GREATEST(q.score, 0), 99) / 10) * 10  AS bucket_lo,
    COUNT(*)                                     AS cnt
FROM qualifications q
JOIN leads l ON l.id = q.lead_id
WHERE l.archived_at IS NULL
GROUP BY l.user_id, date_trunc('day', q.generated_at)::date, (LEAST(GREATEST(q.score, 0), 99) / 10) * 10
WITH DATA;

CREATE UNIQUE INDEX mv_analytics_qual_dist_pk
    ON mv_analytics_qualification_distribution (user_id, day, bucket_lo);
