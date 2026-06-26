-- Revert to the all-time funnel matviews from migration 042.
DROP MATERIALIZED VIEW IF EXISTS mv_analytics_qualification_distribution;
CREATE MATERIALIZED VIEW mv_analytics_qualification_distribution AS
SELECT
    l.user_id,
    (LEAST(GREATEST(q.score, 0), 99) / 10) * 10 AS bucket_lo,
    COUNT(*)                                     AS cnt
FROM qualifications q
JOIN leads l ON l.id = q.lead_id
GROUP BY l.user_id, (LEAST(GREATEST(q.score, 0), 99) / 10) * 10
WITH DATA;

CREATE UNIQUE INDEX mv_analytics_qual_dist_pk
    ON mv_analytics_qualification_distribution (user_id, bucket_lo);

DROP MATERIALIZED VIEW IF EXISTS mv_analytics_sequence_step_conversion;
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
