# Booking-link HITL approval (P0-3)

Status: shipped in v0.19.0 (Telegram booking-link path closed).

## Why

The inbox Telegram bot used to call `bot.Send` the moment `DetectCallAgreement` matched a phrase in an inbound message, attaching the user's calendar URL ("Отлично! Вот ссылка для выбора удобного времени для звонка: …"). The detector is a 30-marker substring match — false positives are easy ("ладно давайте попробуем что-то другое **вместо встречи**" matches "давайте встреч"). When it misfires, the bot has already leaked a booking URL to a lead who never asked for one, and the operator has no chance to intervene.

This is a security and trust gate, not a UX feature: an unrequested booking link in a customer chat looks like spam and ends the conversation faster than no reply at all. Closing the gap was P0-3 in the post-v0.18.1 backlog.

The fix is a human-in-the-loop approval queue. The bot no longer sends auto-drafted replies directly; it parks them in a `pending_replies` table and the operator approves or rejects through the lead detail UI. Only on approve does the dispatcher push the body to Telegram.

## Domain model

```
pending_replies
 ├─ id            UUID PRIMARY KEY
 ├─ user_id       FK users(id)          -- tenant scope on every read
 ├─ lead_id       FK leads(id)          -- which conversation it belongs to
 ├─ channel       'telegram' | 'email'  -- check constraint
 ├─ kind          'booking_link'        -- check constraint (extensible)
 ├─ body          TEXT (CHECK length>0) -- the proposed message
 ├─ status        'pending' | 'approved' | 'sent' | 'rejected'
 ├─ created_at    TIMESTAMPTZ
 ├─ decided_at    TIMESTAMPTZ?          -- set on Approve/Reject
 └─ sent_at       TIMESTAMPTZ?          -- set after dispatcher succeeds
```

Aggregate root is `inbox.PendingReply` (`backend/internal/inbox/pending_reply.go`). The factory `NewPendingReply` enforces non-Nil user/lead ids, valid channel + kind, non-empty trimmed body. The status enum has a state machine encoded in `pendingReplyTransitions`:

```
pending  ──Approve──▶ approved ──MarkSent──▶ sent     (terminal)
   │
   └────Reject────▶ rejected                          (terminal)
```

`Approve(at)`, `Reject(at)` and `MarkSent(at)` are the only writes; each stamps the relevant timestamp atomically with the transition and returns `ErrPendingReplyInvalidTransition` if the move violates the state machine. There is no `TransitionTo` available to callers — domain methods own the timestamp, not the caller.

## HTTP surface

Three routes, all behind the same JWT middleware as the rest of `/api`:

```
GET  /api/leads/{id}/pending-replies     → operator queue for one lead
POST /api/pending-replies/{id}/approve   → 204 on success
POST /api/pending-replies/{id}/reject    → 204 on success
```

`GET` is lead-scoped: the handler gates on `leads.UseCase.OwnsLead` and answers a uniform 404 when the lead belongs to another tenant. `POST` approve / reject delegate to `inbox.PendingReplyUseCase`, which collapses missing-row and cross-tenant into the same `ErrPendingReplyNotFound` sentinel — the handler maps it to 404. Already-decided replies surface as 409.

Response DTO (`PendingReplyResponse`) is intentionally separate from the domain entity so transport concerns (RFC3339 timestamps, JSON tags) stay out of the inbox package surface.

## Wiring

The composition root (`backend/cmd/server/main.go`) has a cycle: `TelegramBot` needs the usecase as `PendingReplyProposer`, the dispatcher needs `tgBot.Bot()`, the usecase needs the dispatcher. The cycle is broken with two runtime setters that mirror the existing `leadsUC.SetSender` pattern:

```go
pendingReplyUC := inbox.NewPendingReplyUseCase(pendingReplyRepo, nil)
// ... register routes referencing pendingReplyUC ...

if tgBot, err := inbox.NewTelegramBot(...); err == nil {
    dispatcher := newTelegramReplyDispatcher(tgBot.Bot(), leadsRepo, inboxLeadAdapter)
    pendingReplyUC.SetDispatcher(dispatcher)
    tgBot.SetPendingProposer(pendingReplyUC)
    go tgBot.Start(ctx)
}
```

Ordering is intentional: `SetDispatcher` runs before `SetPendingProposer`, so any inbound message arriving in the gap finds at worst a logged "dispatcher not configured" error from `Approve`, never a nil-deref panic.

`telegramReplyDispatcher` lives in `cmd/server/reply_dispatcher.go` because it is the cross-context bridge from inbox to Telegram. It exposes a single `Dispatch(ctx, pr) error` and is built from three narrow interfaces (`telegramBotSender`, `leadChatIDFetcher`, `inboxMessageWriter`) for testability. The dispatch order is **send-then-persist**: persist-first risks the UI showing a "sent" row for a message that never reached the customer, which is the worse failure mode.

## Secure defaults

The Telegram bot suppresses the booking-link branch entirely when no `PendingReplyProposer` is wired. There is no instant-send fallback — a misconfigured deploy is louder (booking links don't go out at all) than the alternative (booking links go out without operator review).

Approve persists the entity as `approved` **before** calling the dispatcher. If dispatch fails (Telegram 5xx, network blip), the entity remains in `approved` so the operator can re-approve from the queue; the error propagates to the handler as 500. On dispatch success the usecase transitions to `sent` and persists again.

`PendingReplyRepository` enforces tenant scope on every read method — `GetByID(ctx, userID, id)` returns nil if the row belongs to another user, `ListByLead(ctx, userID, leadID)` filters server-side. The handler relies on this; the usecase belt-and-braces it with its own `loadOwned` helper that collapses the two outcomes into `ErrPendingReplyNotFound`. An attacker who guesses pending-reply UUIDs cannot enumerate other tenants' drafts even if the handler is misrouted.

## Frontend

`frontend/src/components/leads/PendingReplySection.tsx` slots into the lead detail page between `ProspectSuggestionBanner` and `QualificationCard`. It fetches once on mount, renders only `pending` rows, exposes Approve / Reject buttons per row, and disables both buttons during an in-flight decision. Approve failures surface as a `role=alert` message and leave the row in place so the operator can retry.

`onApproved` triggers a messages refetch on the page so the now-delivered body lands in the conversation thread without a manual reload, honouring the aggregated-view preference.

## Future extensions

Things deliberately not in this slice — feeds future issues:

1. **Inbox-list badge** for leads with `pending` rows. Operator visibility today depends on the operator opening each lead detail page. Counts can be queried via `SELECT lead_id, COUNT(*) FROM pending_replies WHERE user_id = $1 AND status = 'pending' GROUP BY lead_id`.

2. **Operator queue page** — a global "everything awaiting my approval" view, similar to the outbound queue. Useful when the operator triages many leads at once.

3. **Email auto-replies**. The email poller currently only creates leads and saves inbound messages — it does not auto-draft. When AI-generated email auto-replies land, they should route through the same queue. The `channel` enum already includes `'email'`; only the dispatcher needs an email branch (Resend/SMTP).

4. **Additional kinds**. `PendingReplyKind` is an open enum (extensible by adding constants + CHECK constraint values). Candidates: `qualification_followup`, `meeting_proposal`, `pricing_response`.

5. **Bulk approve / reject** for power operators. Not strictly necessary at current volume but easy to add over the existing endpoints.

6. **Edit before approve**. Today the operator can only approve the body as-drafted or reject it. Inline editing would require a `PATCH /api/pending-replies/{id}` endpoint that re-validates through the domain factory.

7. **Operator attribution** (`decided_by` column). The audit-log feature already captures per-request `CallMeta`; adding `decided_by uuid REFERENCES users(id)` would surface "who approved this" in the UI. Low effort, deferred to keep the slice focused on the security gate.

## Migration

`backend/migrations/030_pending_replies.up.sql` creates the table and two indexes (`user_id, status, created_at DESC` for the operator queue; `user_id, lead_id, created_at DESC` for the per-lead drill-down). The down migration drops the table.

Verified up → down → up cycle locally before merge.
