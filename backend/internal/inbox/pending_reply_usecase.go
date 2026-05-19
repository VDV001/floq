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
	repo       PendingReplyRepository
	dispatcher ReplyDispatcher
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
func (uc *PendingReplyUseCase) Propose(ctx context.Context, userID, leadID uuid.UUID, channel Channel, kind PendingReplyKind, body string) (*PendingReply, error) {
	pr, err := NewPendingReply(userID, leadID, channel, kind, body)
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
	if err := uc.repo.Update(ctx, pr, PendingReplyStatusPending); err != nil {
		if errors.Is(err, ErrPendingReplyNotFound) {
			return ErrPendingReplyAlreadyDecided
		}
		return fmt.Errorf("persist approved pending reply: %w", err)
	}
	if uc.dispatcher == nil {
		return ErrPendingReplyDispatcherNotConfigured
	}
	if err := uc.dispatcher.Dispatch(ctx, pr); err != nil {
		return fmt.Errorf("dispatch approved pending reply: %w", err)
	}
	if err := pr.MarkSent(time.Now().UTC()); err != nil {
		return fmt.Errorf("mark pending reply sent: %w", err)
	}
	// The persisted row should still be at status=approved (we just
	// set it). A NotFound here would be a serious anomaly — log it
	// rather than convert to AlreadyDecided.
	if err := uc.repo.Update(ctx, pr, PendingReplyStatusApproved); err != nil {
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
