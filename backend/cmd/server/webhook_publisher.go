package main

import (
	"context"
	"encoding/json"
	"log/slog"

	inbox "github.com/daniil/floq/internal/inbox"
	leadsdomain "github.com/daniil/floq/internal/leads/domain"
	webhooksdomain "github.com/daniil/floq/internal/webhooks/domain"
	"github.com/google/uuid"
)

// eventPublisher is the narrow slice of the webhooks usecase the bridge needs:
// fan an event out to a user's subscribed endpoints. *webhooks.UseCase
// satisfies it; kept an interface so the bridge's mapping is unit-testable with
// a fake.
type eventPublisher interface {
	Publish(ctx context.Context, userID uuid.UUID, event webhooksdomain.EventType, data json.RawMessage) (int, error)
}

// webhookEventPublisher bridges domain side-effects in the leads/inbox contexts
// to outgoing webhooks (#181 Phase 2). It implements the per-context observer
// ports structurally (OnLeadCreated / OnLeadQualified / OnLeadArchived /
// OnPendingReplyApproved) and maps each entity to a JSON payload. A nil
// publisher (webhooks disabled) makes every method a safe no-op, so the
// observers can be wired unconditionally and the feature stays gated by
// WEBHOOKS_ENABLED. Enqueue failures are swallowed (logged): emitting a webhook
// must never fail the domain action that triggered it.
type webhookEventPublisher struct {
	pub    eventPublisher
	logger *slog.Logger
}

func newWebhookEventPublisher(pub eventPublisher, logger *slog.Logger) *webhookEventPublisher {
	if logger == nil {
		logger = slog.Default()
	}
	return &webhookEventPublisher{pub: pub, logger: logger}
}

// publish marshals data and fans it out, swallowing (logging) any error.
func (b *webhookEventPublisher) publish(ctx context.Context, userID uuid.UUID, event webhooksdomain.EventType, payload any) {
	if b.pub == nil {
		return // webhooks disabled — no-op
	}
	data, err := json.Marshal(payload)
	if err != nil {
		b.logger.ErrorContext(ctx, "webhooks: marshal event payload", "event", event, "err", err)
		return
	}
	if _, err := b.pub.Publish(ctx, userID, event, data); err != nil {
		b.logger.ErrorContext(ctx, "webhooks: publish failed", "event", event, "user", userID, "err", err)
	}
}

func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// OnLeadCreated emits lead.created for a freshly created inbound lead.
func (b *webhookEventPublisher) OnLeadCreated(ctx context.Context, lead *inbox.InboxLead) {
	b.publish(ctx, lead.UserID, webhooksdomain.EventLeadCreated, map[string]any{
		"id":           lead.ID,
		"channel":      string(lead.Channel),
		"contact_name": lead.ContactName,
		"company":      lead.Company,
		"email":        strOrEmpty(lead.EmailAddress),
		"created_at":   lead.CreatedAt,
	})
}

// OnLeadQualified emits lead.qualified after a lead is scored. Implements
// leadsdomain.QualificationObserver.
func (b *webhookEventPublisher) OnLeadQualified(ctx context.Context, lead *leadsdomain.Lead) {
	b.publish(ctx, lead.UserID, webhooksdomain.EventLeadQualified, map[string]any{
		"id":           lead.ID,
		"status":       string(lead.Status),
		"contact_name": lead.ContactName,
		"company":      lead.Company,
		"email":        strOrEmpty(lead.EmailAddress),
	})
}

// emitInboxLeadQualified emits lead.qualified for the inbox auto-qualification
// path. It is a distinct method from OnLeadQualified(*leadsdomain.Lead) — the
// inbox carries its own InboxLead type, and a single struct can't have two
// methods of the same name. inboxLeadQualifiedObserverFunc adapts it to the
// inbox.LeadQualifiedObserver port.
func (b *webhookEventPublisher) emitInboxLeadQualified(ctx context.Context, lead *inbox.InboxLead) {
	b.publish(ctx, lead.UserID, webhooksdomain.EventLeadQualified, map[string]any{
		"id":           lead.ID,
		"status":       string(lead.Status),
		"contact_name": lead.ContactName,
		"company":      lead.Company,
		"email":        strOrEmpty(lead.EmailAddress),
	})
}

// inboxLeadQualifiedObserverFunc adapts a bridge method to the
// inbox.LeadQualifiedObserver port without a named struct.
type inboxLeadQualifiedObserverFunc func(context.Context, *inbox.InboxLead)

func (f inboxLeadQualifiedObserverFunc) OnLeadQualified(ctx context.Context, lead *inbox.InboxLead) {
	f(ctx, lead)
}

// OnLeadArchived emits lead.archived when a lead leaves the working feeds.
func (b *webhookEventPublisher) OnLeadArchived(ctx context.Context, lead *leadsdomain.Lead) {
	b.publish(ctx, lead.UserID, webhooksdomain.EventLeadArchived, map[string]any{
		"id":          lead.ID,
		"archived_at": lead.ArchivedAt,
	})
}

// OnPendingReplyApproved emits pending_reply.approved when a human approves an
// AI-drafted reply for delivery.
func (b *webhookEventPublisher) OnPendingReplyApproved(ctx context.Context, pr *inbox.PendingReply) {
	b.publish(ctx, pr.UserID, webhooksdomain.EventPendingReplyApproved, map[string]any{
		"id":      pr.ID,
		"lead_id": pr.LeadID,
		"channel": string(pr.Channel),
	})
}

// Compile-time checks that the bridge satisfies the leads qualification port.
var _ leadsdomain.QualificationObserver = (*webhookEventPublisher)(nil)

// compositeQualificationObserver fans a qualification event out to several
// observers (e.g. the 1C counterparty push AND the lead.qualified webhook). The
// leads usecase holds a single QualificationObserver, so the composition root
// wraps the multiple sinks in this composite. Each observer owns its errors.
type compositeQualificationObserver struct {
	observers []leadsdomain.QualificationObserver
	logger    *slog.Logger
}

func newCompositeQualificationObserver(logger *slog.Logger, observers ...leadsdomain.QualificationObserver) *compositeQualificationObserver {
	if logger == nil {
		logger = slog.Default()
	}
	return &compositeQualificationObserver{observers: observers, logger: logger}
}

func (c *compositeQualificationObserver) OnLeadQualified(ctx context.Context, lead *leadsdomain.Lead) {
	for _, o := range c.observers {
		if o != nil {
			c.safeNotify(ctx, o, lead)
		}
	}
}

// safeNotify isolates one observer: a panic in it (e.g. the 1C adapter) must not
// crash the qualification request or skip the remaining observers.
func (c *compositeQualificationObserver) safeNotify(ctx context.Context, o leadsdomain.QualificationObserver, lead *leadsdomain.Lead) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.ErrorContext(ctx, "qualification observer panicked", "lead", lead.ID, "panic", r)
		}
	}()
	o.OnLeadQualified(ctx, lead)
}

var _ leadsdomain.QualificationObserver = (*compositeQualificationObserver)(nil)
