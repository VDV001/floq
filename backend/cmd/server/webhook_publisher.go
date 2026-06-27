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
