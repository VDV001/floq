-- Period-windowed funnel analytics. The MVP matviews (migration 042) were
-- all-time; this redefines both at a grain that supports arbitrary time
-- windows computed at read time, without the NOW() drift a windowed matview
-- would suffer (NOW() lives only in the read query, never in the view/index).
--
-- Key idea: pre-aggregate to the grain where the metric is additive, carry a
-- timestamp, and window with a plain COUNT at read time:
--   * qualification distribution counts (COUNT(*)) are additive → bucket by
--     day; the reader sums days within the window.
--   * conversion uses COUNT(DISTINCT prospect), which is NOT additive across
--     time buckets → instead the view is deduped to one row per
--     (tenant, sequence, step, prospect) carrying when they entered, whether
--     they replied, and whether they advanced. A windowed COUNT over that base
--     is exact for any window with no distinct-additivity hazard.

-- Qualification-score distribution, now bucketed by (tenant, day, width-10 bin).
-- The read-path sums cnt over the days in the requested window, then folds the
-- width-10 bins up to the operator's configured step (a multiple of 10).
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

-- Per-(tenant, sequence, step, prospect) conversion base, deduped to one row
-- per prospect that received the step ('sent'). entered_at is when they
-- received it (earliest send), replied/advanced are booleans. A windowed
-- COUNT(*) / COUNT(*) FILTER over this base gives exact entered/replied/
-- advanced for any time window — the read-path no longer needs COUNT(DISTINCT)
-- (the dedup is baked into the view's grain), so windowing stays additive-safe.
DROP MATERIALIZED VIEW IF EXISTS mv_analytics_sequence_step_conversion;
CREATE MATERIALIZED VIEW mv_analytics_sequence_step_conversion AS
SELECT
    s.user_id,
    om.sequence_id,
    om.step_order,
    om.prospect_id,
    MIN(om.sent_at)                       AS entered_at,
    bool_or(om.replied_at IS NOT NULL)    AS replied,
    bool_or(nxt.prospect_id IS NOT NULL)  AS advanced
FROM outbound_messages om
JOIN sequences s ON s.id = om.sequence_id
LEFT JOIN outbound_messages nxt
    ON  nxt.sequence_id = om.sequence_id
    AND nxt.prospect_id = om.prospect_id
    AND nxt.step_order  = om.step_order + 1
    AND nxt.status      = 'sent'
WHERE om.status = 'sent'
GROUP BY s.user_id, om.sequence_id, om.step_order, om.prospect_id
WITH DATA;

CREATE UNIQUE INDEX mv_analytics_seq_conv_pk
    ON mv_analytics_sequence_step_conversion (user_id, sequence_id, step_order, prospect_id);
