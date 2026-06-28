-- #212 part 2: per-row lease for multi-worker claim.
--
-- Both table-as-queue backlogs (lead_qualification_jobs, webhook_deliveries) are
-- drained by a single worker today: ClaimDue is a plain SELECT and the row stays
-- 'pending' during the (seconds-long, off-transaction) AI / HTTP processing, so a
-- second instance would re-claim and double-process the same row.
--
-- locked_until is the claim lease. A claimer atomically marks the row it takes
-- (locked_until = now() + lease) under FOR UPDATE SKIP LOCKED, and the claim
-- filter skips rows whose lease is still in the future. Workers claim one row at
-- a time and process it immediately, so the lease only has to outlast a single
-- item — independent of batch size. A crashed worker's lease simply expires,
-- making the row reclaimable; no separate recovery sweep is needed. NULL means
-- unleased (the steady state for a row not being processed).
ALTER TABLE lead_qualification_jobs ADD COLUMN locked_until timestamptz;
ALTER TABLE webhook_deliveries ADD COLUMN locked_until timestamptz;
