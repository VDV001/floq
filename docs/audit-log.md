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
  style_check, chat_assist.
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
| `internal/audit/context.go` | `CallMeta` struct + ctx-value helpers. |
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
- **Style-check sub-call attribution** — when a Generate* call enters
  the style-check second pass, the inner Complete should record under
  `style_check` rather than the parent request_type. Today both passes
  end up under the parent type; one row but two underlying calls.
- **Conversation forensics table** — opt-in store for prompt/response
  text linked back to `audit_log.id`, ringed by retention + access
  controls separate from cost data.
