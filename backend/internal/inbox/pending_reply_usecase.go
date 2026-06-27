package inbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ReplyDispatcher delivers an approved PendingReply to the customer
// through the channel-native transport (Telegram Bot API, future
// SMTP/IMAP, etc.). Implementations live in the composition root —
// the usecase only knows the abstract port so the inbox package
// stays free of transport-layer imports.
//
// Returning an error here keeps the entity in the Approved status so
// the operator can retry; the usecase will NOT mark it sent on
// failure.
type ReplyDispatcher interface {
	Dispatch(ctx context.Context, pr *PendingReply) error
}

// --- Sentinels ---

var (
	// ErrPendingReplyNotFound covers both "row does not exist" and
	// "row exists but belongs to another tenant". The two are
	// indistinguishable on purpose so the handler can answer a
	// uniform 404 and not leak existence to attackers.
	ErrPendingReplyNotFound = errors.New("pending reply: not found")

	// ErrPendingReplyAlreadyDecided is returned when the operator
	// tries to Approve or Reject a reply that is no longer in the
	// Pending state. The state machine in the domain rejects the
	// transition itself; this wrapper surfaces a stable sentinel for
	// the handler layer.
	ErrPendingReplyAlreadyDecided = errors.New("pending reply: already decided")

	// ErrPendingReplyDispatcherNotConfigured surfaces a runtime
	// misconfiguration: the usecase was constructed (or post-init
	// SetDispatcher was never called) without a ReplyDispatcher. It
	// is a deliberate sentinel rather than a bare error so the
	// handler maps it to 500 explicitly and ops dashboards can
	// alert on it. In production this should never fire — main.go
	// wires SetDispatcher before SetPendingProposer.
	ErrPendingReplyDispatcherNotConfigured = errors.New("pending reply: dispatcher not configured")
)

// PendingReplyUseCase orchestrates the HITL approval flow for inbox
// auto-drafts. It is the only collaborator that mutates PendingReply
// rows after Save — handlers parse input, call into the usecase, and
// map the response/error to HTTP.
type PendingReplyUseCase struct {
	repo            PendingReplyRepository
	dispatcher      ReplyDispatcher
	classifier      InputClassifier
	tx              TxManager
	approvedEmitter PendingReplyApprovedEmitter
}

// NewPendingReplyUseCase wires the usecase with its persistence port.
// The dispatcher may be nil at construction so the composition root
// can register HTTP routes that reference the usecase before the
// Telegram bot (and therefore the dispatcher built from its
// *tgbotapi.BotAPI) is initialised. Call SetDispatcher once the bot
// is up; Approve returns an explicit error if invoked before then.
func NewPendingReplyUseCase(repo PendingReplyRepository, dispatcher ReplyDispatcher) *PendingReplyUseCase {
	return &PendingReplyUseCase{repo: repo, dispatcher: dispatcher}
}

// SetDispatcher injects the ReplyDispatcher at runtime. Mirrors the
// existing leadsUC.SetSender pattern; necessary to break the
// bot -> usecase -> dispatcher -> bot cycle in the composition root.
func (uc *PendingReplyUseCase) SetDispatcher(d ReplyDispatcher) {
	uc.dispatcher = d
}

// PendingReplyApprovedEmitter writes the pending_reply.approved event
// transactionally, inside the approval transaction (#199): a non-nil error
// aborts the approval (fail-closed), so the approval write and its event row
// commit together or not at all. Implemented in the composition root.
type PendingReplyApprovedEmitter interface {
	EmitPendingReplyApproved(ctx context.Context, pr *PendingReply) error
}

// SetTxManager wires the transaction manager after construction (#199).
func (uc *PendingReplyUseCase) SetTxManager(tx TxManager) { uc.tx = tx }

// SetApprovedEmitter wires the in-transaction pending_reply.approved outbox
// emitter after construction (#199).
func (uc *PendingReplyUseCase) SetApprovedEmitter(e PendingReplyApprovedEmitter) {
	uc.approvedEmitter = e
}

// SetClassifier injects the InputClassifier at runtime (mirrors
// SetDispatcher). When unset, Propose stamps SeverityInfo — the safe
// baseline so a misconfigured deployment never blocks replies, while
// the composition root always wires the real firewall in production.
func (uc *PendingReplyUseCase) SetClassifier(c InputClassifier) {
	uc.classifier = c
}

// Propose constructs a new PendingReply through the domain factory
// (so invariants are enforced) and persists it. Dispatch is
// deliberately skipped — that is the whole point of the HITL gate.
//
// Idempotent against the partial-unique dedup index: if Save reports
// ErrPendingReplyDuplicatePending (a pending row with the same
// content already exists), Propose looks up the previously-enqueued
// entity and returns it instead of failing. This handles the
// Telegram-reconnect double-fire case at the repository boundary so
// callers (the bot, future email auto-reply) do not need their own
// retry-and-suppress logic.
func (uc *PendingReplyUseCase) Propose(ctx context.Context, userID, leadID uuid.UUID, channel Channel, kind PendingReplyKind, body, inboundText string) (*PendingReply, error) {
	// Classify the inbound (untrusted) message — not the outbound body —
	// so the reply carries the firewall verdict of what triggered it. The
	// dispatch gate refuses a reply provoked by a Block-flagged payload.
	severity := SeverityInfo
	if uc.classifier != nil {
		severity = uc.classifier.Classify(inboundText)
	}
	pr, err := NewClassifiedPendingReply(userID, leadID, channel, kind, body, severity)
	if err != nil {
		return nil, err
	}
	saveErr := uc.repo.Save(ctx, pr)
	if saveErr == nil {
		return pr, nil
	}
	if !errors.Is(saveErr, ErrPendingReplyDuplicatePending) {
		return nil, fmt.Errorf("propose pending reply: %w", saveErr)
	}
	// Dedup hit: surface the already-enqueued row. Trimmed body matches
	// what the factory stored on the original Save.
	existing, ferr := uc.repo.FindPendingByContent(ctx, userID, leadID, kind, pr.Body)
	if ferr != nil {
		return nil, fmt.Errorf("propose pending reply: lookup after dedup: %w", ferr)
	}
	if existing == nil {
		// Index said duplicate but the row is gone — race or anomaly.
		// Wrap the sentinel so callers can branch and humans can grep.
		return nil, fmt.Errorf("propose pending reply: dedup hit but no pending row found: %w", saveErr)
	}
	return existing, nil
}

// ListByLead returns every pending-or-decided reply tied to the given
// lead for the given user. The repository scope guarantees no
// cross-tenant leakage.
func (uc *PendingReplyUseCase) ListByLead(ctx context.Context, userID, leadID uuid.UUID) ([]*PendingReply, error) {
	return uc.repo.ListByLead(ctx, userID, leadID)
}

// ListPendingByUser returns every status='pending' row for the user
// joined with the lead snippet the operator queue needs. Passthrough
// to the repository — there is no per-row authorization to apply
// because user_id scoping is already enforced inside the SQL query;
// cross-tenant leakage is impossible at this layer.
func (uc *PendingReplyUseCase) ListPendingByUser(ctx context.Context, userID uuid.UUID) ([]*PendingReplyWithLead, error) {
	return uc.repo.ListPendingByUser(ctx, userID)
}

// BulkDecide applies the same decision to every id in the slice,
// delegating per-row to the existing Approve/Reject usecase methods
// so the optimistic-lock + dispatcher contract is preserved. Per-row
// failures (NotFound, AlreadyDecided, dispatcher 5xx, …) are
// collected into the result slice and do NOT abort the rest — one
// Telegram outage shouldn't poison a fifty-row bulk. The result
// order matches the input order 1-to-1 so callers can correlate.
//
// Top-level error is reserved for request-shape problems
// (ErrBulkDecideEmptyIDs, ErrBulkDecideInvalidDecision) that prevent
// any per-row work from starting; per-row issues never surface here.
func (uc *PendingReplyUseCase) BulkDecide(ctx context.Context, userID uuid.UUID, ids []uuid.UUID, decision BulkDecision) ([]BulkDecideResult, error) {
	if len(ids) == 0 {
		return nil, ErrBulkDecideEmptyIDs
	}
	if !decision.IsValid() {
		return nil, ErrBulkDecideInvalidDecision
	}
	results := make([]BulkDecideResult, 0, len(ids))
	for _, id := range ids {
		// Honour client disconnect mid-bulk: pad remaining ids with
		// the context error so the result slice stays 1-to-1 with the
		// input and the caller can see which rows did not run.
		// Continuing the loop after a cancel would still fire DB
		// updates + dispatcher I/O on a dead context (50 wasted
		// round-trips for a 50-row bulk in the worst case).
		if err := ctx.Err(); err != nil {
			results = append(results, BulkDecideResult{ID: id, Err: err})
			continue
		}
		var err error
		switch decision {
		case BulkDecisionApprove:
			err = uc.Approve(ctx, userID, id)
		case BulkDecisionReject:
			err = uc.Reject(ctx, userID, id)
		}
		results = append(results, BulkDecideResult{ID: id, Err: err})
	}
	return results, nil
}

// Approve transitions the reply into Approved, persists the decision,
// dispatches through the channel-native transport, and (on success)
// marks the entity Sent. A dispatch failure leaves the entity in
// Approved so the operator can retry; the error propagates so the
// handler can surface it. Cross-tenant or missing IDs collapse to
// ErrPendingReplyNotFound (uniform 404).
func (uc *PendingReplyUseCase) Approve(ctx context.Context, userID, id uuid.UUID) error {
	pr, err := uc.loadOwned(ctx, userID, id)
	if err != nil {
		return err
	}
	if err := pr.Approve(time.Now().UTC(), userID); err != nil {
		if errors.Is(err, ErrPendingReplyInvalidTransition) {
			return ErrPendingReplyAlreadyDecided
		}
		return err
	}
	// Optimistic lock: we just loaded the row at status=pending; if
	// the Update returns NotFound the row has been decided by
	// somebody else in the meantime — surface as AlreadyDecided.
	doUpdate := func(ctx context.Context) error {
		if err := uc.repo.Update(ctx, pr, PendingReplyStatusPending); err != nil {
			if errors.Is(err, ErrPendingReplyNotFound) {
				return ErrPendingReplyAlreadyDecided
			}
			return fmt.Errorf("persist approved pending reply: %w", err)
		}
		return nil
	}

	if uc.approvedEmitter != nil && uc.tx != nil {
		// Transactional outbox (#199): the approval write and the
		// pending_reply.approved enqueue commit together or not at all. A
		// failed emit rolls the approval back (fail-closed). The dispatch
		// stays OUTSIDE this transaction — it is an external send that must
		// run only after the approval is durably committed, and its failure
		// must not undo the approval.
		if err := uc.tx.WithTx(ctx, func(txCtx context.Context) error {
			if err := doUpdate(txCtx); err != nil {
				return err
			}
			return uc.approvedEmitter.EmitPendingReplyApproved(txCtx, pr)
		}); err != nil {
			return err
		}
	} else if err := doUpdate(ctx); err != nil {
		// Webhooks disabled (no emitter wired): a plain approval with no event.
		return err
	}
	// Approval is durably committed here (independent of dispatch outcome).
	if uc.dispatcher == nil {
		return ErrPendingReplyDispatcherNotConfigured
	}
	// Re-read after the optimistic-lock Update so a concurrent edit
	// that committed in the gap [loadOwned, Update] does not
	// silently dispatch the pre-edit body (#81). The DB holds the
	// canonical body; our in-memory pr.Body is the load-time
	// snapshot. The Update only writes decision columns, never body,
	// so any edit's body change survives.
	//
	// Re-read failure is degraded: fall back to the in-memory
	// snapshot so a transient DB hiccup does not block delivery of
	// an already-approved row.
	dispatchTarget := pr
	if fresh, ferr := uc.repo.GetByID(ctx, userID, pr.ID); ferr == nil && fresh != nil {
		dispatchTarget = fresh
	}
	if err := uc.dispatcher.Dispatch(ctx, dispatchTarget); err != nil {
		return fmt.Errorf("dispatch approved pending reply: %w", err)
	}
	if err := dispatchTarget.MarkSent(time.Now().UTC()); err != nil {
		return fmt.Errorf("mark pending reply sent: %w", err)
	}
	// The persisted row should still be at status=approved (we just
	// set it). A NotFound here would be a serious anomaly — log it
	// rather than convert to AlreadyDecided.
	if err := uc.repo.Update(ctx, dispatchTarget, PendingReplyStatusApproved); err != nil {
		return fmt.Errorf("persist sent pending reply: %w", err)
	}
	return nil
}

// Reject transitions the reply into the terminal Rejected status and
// persists the decision. No dispatch is performed. Cross-tenant or
// missing IDs collapse to ErrPendingReplyNotFound.
func (uc *PendingReplyUseCase) Reject(ctx context.Context, userID, id uuid.UUID) error {
	pr, err := uc.loadOwned(ctx, userID, id)
	if err != nil {
		return err
	}
	if err := pr.Reject(time.Now().UTC(), userID); err != nil {
		if errors.Is(err, ErrPendingReplyInvalidTransition) {
			return ErrPendingReplyAlreadyDecided
		}
		return err
	}
	if err := uc.repo.Update(ctx, pr, PendingReplyStatusPending); err != nil {
		if errors.Is(err, ErrPendingReplyNotFound) {
			return ErrPendingReplyAlreadyDecided
		}
		return fmt.Errorf("persist rejected pending reply: %w", err)
	}
	return nil
}

// UpdateBody applies a body-only edit on a pending reply scoped by
// user_id. Returns the updated entity on success so the handler can
// surface the new body without a second round-trip. Error contract:
//   - ErrPendingReplyNotFound: missing or cross-tenant id (uniform 404)
//   - ErrPendingReplyAlreadyDecided: row left Pending (transition or
//     edit race; 409 in the handler)
//   - ErrPendingReplyEmptyBody: domain invariant (400 in the handler)
//
// Persistence is optimistic-locked on status=Pending so a concurrent
// approve/reject cannot be silently overwritten.
func (uc *PendingReplyUseCase) UpdateBody(ctx context.Context, userID, id uuid.UUID, body string) (*PendingReply, error) {
	pr, err := uc.loadOwned(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if err := pr.UpdateBody(body); err != nil {
		if errors.Is(err, ErrPendingReplyNotEditable) {
			return nil, ErrPendingReplyAlreadyDecided
		}
		return nil, err
	}
	if err := uc.repo.UpdateBody(ctx, pr, PendingReplyStatusPending); err != nil {
		if errors.Is(err, ErrPendingReplyNotFound) {
			return nil, ErrPendingReplyAlreadyDecided
		}
		return nil, fmt.Errorf("persist pending reply body: %w", err)
	}
	return pr, nil
}

// loadOwned fetches a reply scoped by user_id and collapses
// "not found" and "not owned" into ErrPendingReplyNotFound so callers
// cannot leak existence cross-tenant by inspecting the error.
func (uc *PendingReplyUseCase) loadOwned(ctx context.Context, userID, id uuid.UUID) (*PendingReply, error) {
	pr, err := uc.repo.GetByID(ctx, userID, id)
	if err != nil {
		return nil, fmt.Errorf("load pending reply: %w", err)
	}
	if pr == nil {
		return nil, ErrPendingReplyNotFound
	}
	return pr, nil
}
