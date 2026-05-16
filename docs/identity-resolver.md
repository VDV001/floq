# Identity resolver — Phase 2 multi-source aggregation (#27)

Status: shipped in v0.15.0 (foundation), v0.16.0 (wiring + backfill), v0.17.0 (lead detail UI + toggle).

## Why

When the same prospect reaches a user across multiple channels — email, Telegram, prior outbound campaign — the legacy inbox shows them as N independent leads. The operator has to remember "Bob from Acme wrote me on Telegram last week, now he replied to my cold email" and manually merge contexts in their head. The identity resolver removes that burden by collapsing all channel-native handles for one human into a single `Identity` row.

## Domain model

```
identities (id, user_id, email, phone, telegram_username, created_at)
   ▲                                                         ▲
   │ lead_identities (lead_id, identity_id, linked_at)        │
   └──────────────────────────────────────────────────────────┤
                                                              │
                                          prospect_identities │
                                                              │
                                       (prospect_id, identity_id, linked_at)
```

`identities` carries the **canonical** identifier shape — lowercase+trim for email and telegram, digits with optional leading `+` for phone. The `internal/normalize` kernel is the single source of truth for the canonicalization rules; both factories (`domain.NewIdentity`) and the resolver re-normalize defensively so the byte representation stays stable regardless of caller hygiene.

Partial unique indexes per identifier (scoped to `user_id`) enforce one identity per handle per tenant, while still permitting NULL handles so an identity can carry only some of the three columns.

## Resolver

`leads.IdentityResolver.Resolve(ctx, userID, email, phone, tg)` is the single entry point:

1. Canonicalize each identifier through `internal/normalize`.
2. Reject `ErrIdentityNoIdentifiers` if all three end up empty.
3. Walk a **deterministic priority chain** — `email > phone > telegram_username` — and return the first existing match.
4. If nothing matches, call `domain.NewIdentity` and `repo.Save`.

The resolver does **not** enrich an existing identity with newly-supplied identifiers — if Bob arrives by email today and by Telegram tomorrow, the first call creates `{email: "bob@..."}` and the second returns that same row even though we now know his Telegram username. This is intentional for Phase 2: merge/ownership semantics for partially-overlapping identities is a Phase 3 concern.

## Wiring

| Surface | Linker call | Where |
|---|---|---|
| Inbox email poller | `LinkLeadToIdentity(ctx, userID, leadID, fromEmail, "", "")` | after CreateLead in `processEmail` |
| Inbox telegram bot | `LinkLeadToIdentity(ctx, userID, leadID, "", "", username)` | after CreateLead in `handleMessage` |
| Prospects CSV import | `LinkProspectToIdentity(ctx, userID, prospectID, email, phone, tg)` | after CreateProspectsBatch in `ImportCSV` |
| Startup backfill | both, over every legacy `leads` + `prospects` row | one-shot goroutine in `cmd/server/main.go` |

The composition root holds a single `identityLinkerAdapter` that satisfies both narrow ports (`inbox.IdentityLinker`, `prospects.IdentityLinker`) and routes through one resolver + one repository — that's how a lead-by-email and a prospect-by-telegram for the same person converge on a single `identities` row.

`LinkLead`/`LinkProspect` use `INSERT ... ON CONFLICT DO NOTHING`, which makes the backfill idempotent: re-running over the same legacy table produces no duplicate link rows.

## Read paths (v0.17.0)

- `GET /api/leads/:id` includes an optional `identity` field with `{ id, email, phone, telegram_username, linked_lead_ids }` when the lead has been linked.
- `GET /api/leads/:id/messages?aggregated=true` merges messages from every lead pointing at the identity, sorted by `sent_at ASC`. Omitting the query param or passing `false` keeps the legacy single-lead behaviour.
- `user_settings.aggregated_inbox_view` (migration 027, default `TRUE`) controls which mode the frontend uses for a given user. The toggle lives in the Settings page; saving fires immediately (no batched-Save flow).

## Graceful degradation

Every identity-aware code path **must not break** the host flow when the identity backend is unhealthy:

- Linker errors in inbox/prospects → log + swallow (lead still lands).
- Identity fetch errors in `GetLeadView` → fall back to lead-only view (no 500).
- Per-row failures in backfill → log + continue walking.
- Aggregated timeline partial fetch failures → return whatever leads we managed to reach.

The dual-default pattern (SQL `DEFAULT TRUE` on `aggregated_inbox_view` *and* Go-side fallback in `repository.GetSettings`) ensures that a user without a `user_settings` row sees the same behaviour as one with a freshly inserted row — no implicit opt-out hiding behind ErrNoRows.

## Observability

All identity-side log lines flow through an injectable `*slog.Logger` (option `WithLogger` / `WithTelegramLogger` / `WithBackfillLogger`). Levels:

- `WARN` for swallowed errors that change observable behaviour (lead not linked, aggregated timeline degraded).
- `INFO` for happy-path startup events (backfill started/finished).
- No PII in messages — only IDs and bounded provider names.

## Out of scope (Phase 3+)

- Operator-driven manual merge: "this lead is actually the same person as that one".
- Enrichment of an existing identity when a newer call supplies more identifiers.
- Conflict resolution when two distinct identities each match a different supplied identifier in the same call.
- Cross-channel reply suggestions / sentiment aggregation.

These are tracked in the integration audit and will land as separate issues once the foundation has lived in production for a release cycle.
