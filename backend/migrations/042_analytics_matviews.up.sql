-- Analytics read-path: pre-aggregated funnel projections served from the
-- isolated analytics pool. These materialized views move the heavy GROUP BY
-- aggregations off the OLTP query path; a background cron refreshes them
-- CONCURRENTLY, which requires a UNIQUE index on each view (below).
--
-- All-time aggregations (no time window): a matview can't carry a moving
-- NOW() window without drifting on every refresh, and the MVP dashboard
-- shows cumulative funnel health. Period-scoped variants are a later
-- iteration (would need time-bucketed views).

-- Qualification-score distribution, bucketed at the finest grain (width 10)
-- per tenant. The read-path folds these width-10 bins up to the operator's
-- configured step (a multiple of 10), so the view stays fixed while the
-- bucket size remains configurable. score is 0-100; 100 folds into the
-- 90-100 bin via LEAST(score,99).
CREATE MATERIALIZED VIEW mv_analytics_qualification_distribution AS
SELECT
    l.user_id,
    (LEAST(GREATEST(q.score, 0), 99) / 10) * 10 AS bucket_lo,
    COUNT(*)                                     AS cnt
FROM qualifications q
JOIN leads l ON l.id = q.lead_id
GROUP BY l.user_id, (LEAST(GREATEST(q.score, 0), 99) / 10) * 10
WITH DATA;

-- REFRESH ... CONCURRENTLY requires a unique index covering every row.
CREATE UNIQUE INDEX mv_analytics_qual_dist_pk
    ON mv_analytics_qualification_distribution (user_id, bucket_lo);

-- Per-sequence step conversion funnel: for each (tenant, sequence, step) the
-- number of prospects that received the step (entered), replied to it
-- (replied), and went on to receive the next step (advanced). Entered counts
-- delivered ('sent') messages so the funnel reflects what actually went out;
-- rates are computed in the read-path to keep the view integer-pure.
CREATE MATERIALIZED VIEW mv_analytics_sequence_step_conversion AS
SELECT
    s.user_id,
    om.sequence_id,
    om.step_order,
    COUNT(DISTINCT om.prospect_id) FILTER (WHERE om.status = 'sent')                                AS entered,
    COUNT(DISTINCT om.prospect_id) FILTER (WHERE om.status = 'sent' AND om.replied_at IS NOT NULL)   AS replied,
    COUNT(DISTINCT nxt.prospect_id)                                                                  AS advanced
FROM outbound_messages om
JOIN sequences s ON s.id = om.sequence_id
LEFT JOIN outbound_messages nxt
    ON  nxt.sequence_id = om.sequence_id
    AND nxt.prospect_id = om.prospect_id
    AND nxt.step_order  = om.step_order + 1
    AND nxt.status      = 'sent'
GROUP BY s.user_id, om.sequence_id, om.step_order
WITH DATA;

CREATE UNIQUE INDEX mv_analytics_seq_conv_pk
    ON mv_analytics_sequence_step_conversion (user_id, sequence_id, step_order);
