# ADR: Analytics read-path isolation (Postgres matviews + dedicated pool)

**Status:** Accepted (MVP shipped) · **Date:** 2026-06-25

## Context

Funnel analytics (qualification-score distribution, per-sequence step
conversion) require heavy `GROUP BY` aggregations over `qualifications`,
`leads`, `outbound_messages` and `sequences`. Running these inline on the
OLTP query path risks contending with the transactional workload (inbound
message handling, lead/prospect writes, sequence dispatch) as data grows.

We want to isolate the analytics read path from OLTP **without** adding a
separate analytical datastore (ClickHouse et al.) at this stage — the data
volume does not yet justify the operational cost.

## Decision

1. **Materialized views in Postgres** (`migration 042`) pre-compute the
   funnel aggregations off the hot path:
   - `mv_analytics_qualification_distribution` — score histogram at width-10
     bins per tenant; the read-path folds bins up to a configurable step.
   - `mv_analytics_sequence_step_conversion` — entered → replied → advanced
     per (tenant, sequence, step).
   Each view carries a `UNIQUE` index so a background cron can
   `REFRESH MATERIALIZED VIEW CONCURRENTLY` without blocking readers.

2. **Dedicated read-only pool** (`analyticsPool`) built from its own DSN
   (`ANALYTICS_DATABASE_URL`). All analytics reads go through it; the OLTP
   primary pool is untouched by analytics traffic.

   > **Honest qualifier:** in the MVP the read replica is *emulated* by a
   > separate pool/config pointing at the **same** Postgres instance
   > (`ANALYTICS_DATABASE_URL` defaults to `DATABASE_URL`). A real replica is
   > a production deployment concern — point the env var at the replica's
   > DSN and no application code changes.

3. **Background refresh cron** (`analytics.RefreshCron`) rebuilds the views
   on `ANALYTICS_REFRESH_INTERVAL` (default 5m) via a `context.Context`-bound
   goroutine that shuts down cleanly with the server.

4. **DTO-only read models.** The funnel projections are `*DTO` value types
   (public fields, no invariants) behind a `FunnelReader` port defined in the
   consumer (usecase); the matview-backed implementation lives in the
   repository. No domain entities — these are read-side projections.

## Scale-path (when we outgrow this)

When matview refresh latency or OLTP-instance contention becomes the
bottleneck (e.g. refreshes that can't keep up at the configured interval, or
analytics load measurably affecting transactional p99), the next step is
**CDC from Postgres into a columnar store**:

- Stream changes from the OLTP Postgres via logical replication / CDC
  (e.g. **PeerDB** or Debezium) into **ClickHouse**.
- Move the heavy funnel/time-series aggregations to ClickHouse; keep the
  `FunnelReader` port stable and swap the implementation behind it.

This is **explicitly out of scope** for the current work — recorded here so
the migration path is known, not built prematurely. No CDC/ClickHouse code
exists yet.

## Consequences

- Aggregations are minutes-stale (refresh interval); acceptable for an
  operator funnel dashboard, not for real-time figures.
- Funnel matviews are all-time (no time window) in the MVP; period-scoped
  variants would need time-bucketed views.
- The `FunnelReader` seam means the scale-path swap (matview → ClickHouse)
  is an implementation change, not an interface change.
