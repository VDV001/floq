# AI Cost Audit Log

Cross-cutting cost-attribution ledger for every AI provider call. Built
to close the final acceptance box on issue #25 (image-analysis cost
recorded per attachment) and provide the foundation for per-user cost
reports.

## Scope

One row in `audit_log` per OpenAI / Anthropic / Ollama call —
both `Provider.Complete` and `VisionProvider.AnalyzeImage`. The row
captures:

- **Who** issued the call: `user_id` (NOT NULL).
- **What** it was about: `lead_id` and `prospect_id` (both nullable).
- **Why**: `request_type` enum — qualification, draft_reply, cold_message,
  telegram_message, telegram_reply, call_brief, followup, image_analysis,
  style_check, chat_assist, enrichment.
- **What it ran**: `provider` (matches `Provider.Name()` verbatim) and
  `model` (the concrete resolved model, not the per-mode default).
- **What it cost**: `input_tokens`, `output_tokens`, `total_tokens`,
  `cost_usd_micro` (int64 micro-USD), `latency_ms`.
- **How it ended**: `status` ∈ {success, error}, `error_message` (XOR
  constraint: present iff status=error).

No prompt or response content is persisted. Privacy hazard plus
row-size blowup; if conversation forensics is needed later it belongs in
a separate channel-scoped table.

## Architecture

```
business code (use case)
        │
        │  ctx = audit.ContextWithCallMeta(ctx, CallMeta{...})
        ▼
AIClient methods (Qualify / DraftReply / Generate* / AnalyzeImage)
        │
        ▼
audit.RecordingProvider  ──── computes cost via pricing table ────►  AsyncRecorder.Record
        │                                                                       │
        ▼                                                                       │ buffered chan
inner ai.Provider (OpenAI / Anthropic / Ollama)                                 ▼
                                                                          worker goroutine
                                                                                │
                                                                                │  pgx.CopyFrom batch
                                                                                ▼
                                                                          audit_log table
```

### Layers

| File | Role |
|------|------|
| `internal/audit/domain/entry.go` | `AuditLogEntry` aggregate + `NewEntry` factory + invariants. Immutable: no mutators (compliance integrity). |
| `internal/audit/domain/ports.go` | `AuditRepository` (Save batch) + `Recorder` (Record non-blocking). |
| `internal/audit/repository.go` | pgx impl using `pgx.CopyFrom` for atomic bulk insert. |
| `internal/audit/recorder.go` | `AsyncRecorder`: bounded chan + single worker + drop-on-full. |
| `internal/audit/recording_provider.go` | Decorator over `ai.Provider` (+ `ai.VisionProvider` when inner supports it). Pulls `CallMeta` from ctx, computes cost, hands an `Entry` to the recorder. |
| `internal/audit/domain/call_meta.go` | `CallMeta` struct + ctx-value helpers (`ContextWithCallMeta`, `CallMetaFromContext`, `WithRequestType`). Lives in `domain/` so business packages can import the DTO surface without pulling in the recording machinery. |
| `internal/audit/pricing.go` | Static (provider, model) → unit price table. |

### CA / DDD positioning

`audit` is a cross-cutting concern, not a peer bounded context. The
pure-DTO `CallMeta` surface is intentionally imported from every use
case that needs attribution — logging is structured the same way and
nobody calls that a layering violation. Inner `audit/domain` has no
dependencies on business code, so the import direction stays one-way.

## Cost model

### Unit

Money is stored as `int64 micro-USD` (USD × 1 000 000). Integer
arithmetic — no float drift in aggregations. The pricing table caches
unit prices in the same scale.

### Pricing source

Hard-coded constants in `pricing.go` with the capture date in the file
header. Updating procedure:

1. Bump the constants.
2. Update the captured date.
3. Add a chronicles entry.

Costs already recorded retain their original values — we store the
computed cost, not a recompute pointer. This is intentional: cost
reports must be reproducible.

### Floor semantics

`cost_micro = tokens * unit_price_per_million / 1_000_000` uses int64
floor division. A call billing less than 1 micro-USD per dimension
rounds to 0. The audit log under-reports rather than inflates.

### Unknown models

`CostMicroUSD(provider, model, ...)` returns `(0, false)` when the
pair is absent from the pricing table. The recording layer still
writes the row (with cost=0 and the actual model name) so the gap
shows up in reports. Update the table to close the gap.

### Ollama special case

Provider "ollama" returns `(0, true)` regardless of model — the spend
lives on local compute, not a paid API.

## Async-recording semantics

The recorder is configured at boot in `cmd/server/main.go`. Defaults:

- `bufferSize: 1024` — buffered chan capacity.
- `batchSize: 50` — flush when this many entries accumulate.
- `flushInterval: 5s` — flush even if `batchSize` not yet reached.

### Drop policy

Buffer full → non-blocking send drops the entry, increments
`recorder.Dropped()`, warn-logs with `dropped_total`. The AI hot path
never blocks on audit. Dropped count is observable from process
metrics (TODO: wire to Prometheus once metrics layer is in).

### Graceful shutdown

`Stop(ctx)` signals the worker, drains the channel non-blocking,
flushes one final batch via `repo.Save(context.Background())`. Save
uses `context.Background()` so an HTTP request's cancellation doesn't
lose its own cost row — the audit-log write must succeed regardless
of the originating ctx's lifecycle.

Entries still in the buffer past the shutdown deadline are dropped
silently — better than blocking shutdown on a stuck Postgres write.
Documented trade-off: observed-cost ledger, not financial truth.

## Migration

`028_audit_log.up.sql`:

- `user_id` FK → users(id) `ON DELETE CASCADE` — GDPR erasure pulls
  the cost history with the user.
- `lead_id` / `prospect_id` FK → `ON DELETE SET NULL` — per-user
  totals survive entity deletion.
- CHECK constraints duplicate every domain invariant (XOR on
  error_message, non-negative tokens/cost/latency, request_type and
  status enums).
- `(user_id, created_at DESC)` index for the primary "cost report for
  a user" access pattern; partial indexes per `lead_id` /
  `prospect_id` for per-entity drilldowns.

`036_audit_log_daily.up.sql` adds the retention rollup table — see
[Retention & rotation](#retention--rotation-audit_log_daily).

## Style-check sub-call attribution

When `AIClient.GenerateColdMessage` (or the Telegram / followup variants)
is called with style-check enabled, the inner second LLM pass that
judges the draft fires under a derived ctx:
`auditdomain.WithRequestType(ctx, RequestTypeStyleCheck)`. Attribution
inherits user/lead/prospect from the parent but stamps
`request_type='style_check'` so cost reports break down "how much of
this user's spend is the style critic vs. the actual draft."

## Operations runbook

### "Audit rows are missing for user X"

1. **Did `auditdomain.ContextWithCallMeta` run before the AI call?**
   `grep -rn "ContextWithCallMeta" internal/` must show one wrapper per
   call site (leads, sequences, inbox, reminders, chat). When meta is
   absent, `RecordingProvider.record` logs `"audit: AI call missing
   meta context, skipping audit row"` at warn. Search prod logs for
   that exact string.
2. **Did the recorder buffer fill up?** Look for `"audit recorder:
   buffer full, dropping entry"` with a `dropped_total` counter in the
   structured log. If non-zero, raise `WithBufferSize` in
   `cmd/server/main.go` and redeploy.
3. **Did the repo write fail?** Look for `"audit recorder: save
   failed"` warnings — Postgres timeout / connection reset. The worker
   keeps running; that batch is lost.

### "Cost numbers look wrong"

- Pricing constants live in `internal/audit/pricing.go` — diff against
  the provider's public pricing page; the file header records the
  capture date.
- Floor-division means very small calls round to 0 — observed cost is
  always an under-report, never an over-report.
- Unknown (provider, model) pair → cost=0 with `model="unknown"`
  written to the row. Filter on that condition to spot attribution
  gaps to fix.

### "Shutdown takes too long"

- `AsyncRecorder.Stop` drains the buffer inside the HTTP server's 10s
  shutdown context. If you see `audit recorder stop: context deadline
  exceeded`, the buffer was full **and** a repo write was in flight at
  SIGTERM. Increase the shutdown budget in `cmd/server/main.go` or
  accept the loss (audit log is the observed-cost ledger, not
  financial truth).

### Privacy & retention

- `error_message` is sanitised before storage. `sanitizeErrorMessage`
  in `internal/audit/recording_provider.go` runs these regex
  redactions, each replacing matches with `[REDACTED]`:
  - email addresses;
  - E.164-ish phone numbers (`+` prefix, 7–15 digits);
  - `sk-…` API keys (OpenAI-style);
  - `Bearer …` tokens;
  - `AKIA…` AWS access keys;
  - `Authorization: …` header values (case-insensitive);
  - basic-auth userinfo in URLs (`https://user:pass@host`);
  - bare IPv4 addresses.

  The redacted string is then capped at 256 bytes, walking back to a
  rune boundary so non-ASCII errors don't break mid-codepoint.

  **Residual risk (not redacted):** free-form names, postal addresses,
  CRM identifiers (lead IDs, ticket numbers), IPv6 addresses, structured
  org names. The assumption is that providers do not echo those in error
  strings. If a real incident contradicts that, add a regex to
  `piiPatterns` and a covering case to `sanitize_test.go`.
- No prompt or response content is stored — privacy hazard + row-size
  blowup.
- `DELETE FROM users WHERE id = ?` cascades to `audit_log` (GDPR
  erasure). Lead / prospect deletion leaves the row with NULL FK so
  per-user totals stay correct. Locked by integration test
  `TestAcceptance_UserDeletionCascadesToAuditLog`.

## Retention & rotation (audit_log_daily)

The per-call ledger grows unbounded (~200 bytes/row; 10k calls/day ≈
730 MB/year). Migration `036_audit_log_daily.up.sql` adds a day-granular
rollup, and a daily cron (`audit.RetentionCron`) ages detail out of
`audit_log` into it.

**Strategy — aggregate-then-delete (#101).** Cost trends are preserved;
per-call detail beyond the window is shed. Chosen over hard-delete
(loses trends) and partitioning (operational complexity).

- **Schema.** `audit_log_daily (day, user_id, provider, model,
  request_type, total_calls, total_cost_usd_micro, total_input_tokens,
  total_output_tokens)`, PK on the five dimensions. `user_id` FK
  `ON DELETE CASCADE` mirrors `audit_log` (GDPR erasure pulls the
  rollup too). No `request_type` CHECK — it is a rollup of
  already-validated source rows, and coupling it to the enum would force
  a constraint bump here on every extension (cf. migration 029). Extra
  `(user_id, day)` index for the cost-summary read path (PK leads with
  `day`).
- **Cron.** `audit.RetentionCron` mirrors `onec.ReconcileCron`: runs once
  on startup, then every `AUDIT_RETENTION_INTERVAL` (default `24h`),
  stops on ctx cancel. `RetentionUseCase` turns `AUDIT_RETENTION_DAYS`
  (default `30`) into the cut-off `now() - days`.
- **Repository.** `AggregateAndPurgeOlderThan(ctx, threshold)` runs the
  roll-up and delete as ONE data-modifying CTE
  (`DELETE … RETURNING` → `GROUP BY` → `INSERT … ON CONFLICT DO UPDATE`
  accumulate), so the aggregated rows are exactly the deleted rows — no
  window where a row is purged-but-not-counted. Idempotent: a second run
  finds nothing < threshold and leaves buckets untouched. Day bucket =
  UTC calendar date of `created_at` (session-timezone independent).
- **Cost summary.** `Repository.CostSummary` `UNION ALL`s `audit_log`
  (recent detail) with `audit_log_daily` (aggregated history) and sums in
  an outer `GROUP BY`. The two are disjoint (a row lives in exactly one),
  so no double-count. The daily side is filtered by UTC day, so
  aggregated history is **day-granular**: a from/to landing mid-day on an
  already-rolled-up day pulls that whole day's bucket — the accepted
  cost of shedding per-call precision past the window. Recent data, where
  sub-day precision matters, is still served from `audit_log`.

Tuning: `AUDIT_RETENTION_DAYS` (window), `AUDIT_RETENTION_INTERVAL` (cron
cadence). Setting the window very low loses detail faster; the rollup
keeps cost trends regardless.

## Phase 3 backlog

Out of scope for issue #25 closure, on the roadmap once usage justifies:

- **HTTP report endpoint** — `GET /api/audit/cost-summary?from=...&to=...`
  with per-request-type and per-model breakdown.
- **Per-user budget alerts** — surface threshold breaches to the
  notifier (Telegram / email).
- **Retention policy** — partition by month, archive or roll up rows
  older than 90 days.
- **Aggregation queries** — `AggregateByUser` on the repository
  returning `SUM(cost) GROUP BY (provider, model, request_type)`.
- **Prometheus metrics** — `audit_recorder_dropped_total`,
  `audit_recorder_batch_size`, `audit_recorder_flush_latency_seconds`.
- **Inbound Telegram conversation replies wiring** — `AIClient.Generate-
  TelegramReply` attribution is in place (RequestTypeTelegramReply +
  WithRequestType override) but no production caller invokes it yet.
  Reserved for the inbound-bot conversation flow.
- **Conversation forensics table** — opt-in store for prompt/response
  text linked back to `audit_log.id`, ringed by retention + access
  controls separate from cost data.
