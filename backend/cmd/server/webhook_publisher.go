package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	inbox "github.com/daniil/floq/internal/inbox"
	leadsdomain "github.com/daniil/floq/internal/leads/domain"
	"github.com/daniil/floq/internal/outbound"
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

// webhookEventPublisher bridges domain state changes (leads/inbox/outbound) to
// the outgoing-webhooks outbox. It implements the per-context transactional
// Emitter ports (#199): each Emit* method enqueues a delivery via the webhooks
// usecase, whose repository writes through db.ConnFromCtx and therefore joins
// the caller's transaction. A returned error aborts that transaction
// (fail-closed), so the domain change and its event row commit together or not
// at all. The bridge is only wired when webhooks are enabled, so pub is non-nil
// in practice; the nil-guard keeps it a safe no-op otherwise.
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

// emit marshals data and fans it out to the user's subscribed endpoints,
// returning any error so the caller's transaction can roll back (#199).
func (b *webhookEventPublisher) emit(ctx context.Context, userID uuid.UUID, event webhooksdomain.EventType, payload any) error {
	if b.pub == nil {
		return nil // webhooks disabled — no-op
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webhooks: marshal %s payload: %w", event, err)
	}
	if _, err := b.pub.Publish(ctx, userID, event, data); err != nil {
		return fmt.Errorf("webhooks: emit %s: %w", event, err)
	}
	return nil
}

func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// EmitLeadCreated enqueues lead.created for a freshly created inbound lead.
// Implements inbox.LeadCreatedEmitter.
func (b *webhookEventPublisher) EmitLeadCreated(ctx context.Context, lead *inbox.InboxLead) error {
	return b.emit(ctx, lead.UserID, webhooksdomain.EventLeadCreated, map[string]any{
		"id":           lead.ID,
		"channel":      string(lead.Channel),
		"contact_name": lead.ContactName,
		"company":      lead.Company,
		"email":        strOrEmpty(lead.EmailAddress),
		"created_at":   lead.CreatedAt,
	})
}

// EmitLeadQualified enqueues lead.qualified after a lead is scored on the
// leads-context (manual /qualify) path. Implements leadsdomain.QualificationEmitter.
func (b *webhookEventPublisher) EmitLeadQualified(ctx context.Context, lead *leadsdomain.Lead) error {
	return b.emit(ctx, lead.UserID, webhooksdomain.EventLeadQualified, map[string]any{
		"id":           lead.ID,
		"status":       string(lead.Status),
		"contact_name": lead.ContactName,
		"company":      lead.Company,
		"email":        strOrEmpty(lead.EmailAddress),
	})
}

// emitInboxLeadQualified enqueues lead.qualified for the inbox auto-qualification
// path. It is a distinct method from EmitLeadQualified(*leadsdomain.Lead) — the
// inbox carries its own InboxLead type, and a single struct can't have two
// methods of the same name. inboxLeadQualifiedEmitterFunc adapts it to the
// inbox.LeadQualifiedEmitter port.
func (b *webhookEventPublisher) emitInboxLeadQualified(ctx context.Context, lead *inbox.InboxLead) error {
	return b.emit(ctx, lead.UserID, webhooksdomain.EventLeadQualified, map[string]any{
		"id":           lead.ID,
		"status":       string(lead.Status),
		"contact_name": lead.ContactName,
		"company":      lead.Company,
		"email":        strOrEmpty(lead.EmailAddress),
	})
}

// inboxLeadQualifiedEmitterFunc adapts a bridge method to the
// inbox.LeadQualifiedEmitter port without a named struct.
type inboxLeadQualifiedEmitterFunc func(context.Context, *inbox.InboxLead) error

func (f inboxLeadQualifiedEmitterFunc) EmitLeadQualified(ctx context.Context, lead *inbox.InboxLead) error {
	return f(ctx, lead)
}

// EmitLeadArchived enqueues lead.archived when a lead leaves the working feeds.
// Implements leadsdomain.LeadArchivedEmitter.
func (b *webhookEventPublisher) EmitLeadArchived(ctx context.Context, lead *leadsdomain.Lead) error {
	return b.emit(ctx, lead.UserID, webhooksdomain.EventLeadArchived, map[string]any{
		"id":          lead.ID,
		"archived_at": lead.ArchivedAt,
	})
}

// EmitPendingReplyApproved enqueues pending_reply.approved when a human approves
// an AI-drafted reply for delivery. Implements inbox.PendingReplyApprovedEmitter.
func (b *webhookEventPublisher) EmitPendingReplyApproved(ctx context.Context, pr *inbox.PendingReply) error {
	return b.emit(ctx, pr.UserID, webhooksdomain.EventPendingReplyApproved, map[string]any{
		"id":      pr.ID,
		"lead_id": pr.LeadID,
		"channel": string(pr.Channel),
	})
}

// EmitSequenceCompleted enqueues sequence.completed when a prospect's sequence
// run finishes sending its last message. Implements
// outbound.SequenceCompletionEmitter.
func (b *webhookEventPublisher) EmitSequenceCompleted(ctx context.Context, ev outbound.SequenceCompletion) error {
	return b.emit(ctx, ev.UserID, webhooksdomain.EventSequenceCompleted, map[string]any{
		"prospect_id": ev.ProspectID,
		"sequence_id": ev.SequenceID,
	})
}

// Compile-time checks that the bridge satisfies every transactional outbox port.
var (
	_ leadsdomain.QualificationEmitter   = (*webhookEventPublisher)(nil)
	_ leadsdomain.LeadArchivedEmitter    = (*webhookEventPublisher)(nil)
	_ inbox.LeadCreatedEmitter           = (*webhookEventPublisher)(nil)
	_ inbox.PendingReplyApprovedEmitter  = (*webhookEventPublisher)(nil)
	_ outbound.SequenceCompletionEmitter = (*webhookEventPublisher)(nil)
	_ inbox.LeadQualifiedEmitter         = inboxLeadQualifiedEmitterFunc(nil)
)
