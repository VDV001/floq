package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	inbox "github.com/daniil/floq/internal/inbox"
	leadsdomain "github.com/daniil/floq/internal/leads/domain"
	webhooksdomain "github.com/daniil/floq/internal/webhooks/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeEventPublisher struct {
	calls []publishedEvent
}

type publishedEvent struct {
	userID uuid.UUID
	event  webhooksdomain.EventType
	data   json.RawMessage
}

func (f *fakeEventPublisher) Publish(_ context.Context, userID uuid.UUID, event webhooksdomain.EventType, data json.RawMessage) (int, error) {
	f.calls = append(f.calls, publishedEvent{userID, event, data})
	return 1, nil
}

func quietWebhookPublisher(p eventPublisher) *webhookEventPublisher {
	return newWebhookEventPublisher(p, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestWebhookPublisher_OnLeadCreated(t *testing.T) {
	pub := &fakeEventPublisher{}
	b := quietWebhookPublisher(pub)
	userID := uuid.New()
	email := "ivan@acme.ru"
	lead := &inbox.InboxLead{ID: uuid.New(), UserID: userID, Channel: inbox.ChannelTelegram,
		ContactName: "Ivan", Company: "Acme", EmailAddress: &email}

	b.OnLeadCreated(context.Background(), lead)

	require.Len(t, pub.calls, 1)
	assert.Equal(t, webhooksdomain.EventLeadCreated, pub.calls[0].event)
	assert.Equal(t, userID, pub.calls[0].userID)
	assert.Contains(t, string(pub.calls[0].data), lead.ID.String())
	assert.Contains(t, string(pub.calls[0].data), "ivan@acme.ru")
}

func TestWebhookPublisher_OnLeadQualified(t *testing.T) {
	pub := &fakeEventPublisher{}
	b := quietWebhookPublisher(pub)
	userID := uuid.New()
	lead := &leadsdomain.Lead{ID: uuid.New(), UserID: userID, ContactName: "Ann",
		Status: leadsdomain.StatusQualified}

	b.OnLeadQualified(context.Background(), lead)

	require.Len(t, pub.calls, 1)
	assert.Equal(t, webhooksdomain.EventLeadQualified, pub.calls[0].event)
	assert.Equal(t, userID, pub.calls[0].userID)
	assert.Contains(t, string(pub.calls[0].data), "qualified")
}

func TestWebhookPublisher_OnLeadArchived(t *testing.T) {
	pub := &fakeEventPublisher{}
	b := quietWebhookPublisher(pub)
	userID := uuid.New()
	lead := &leadsdomain.Lead{ID: uuid.New(), UserID: userID}

	b.OnLeadArchived(context.Background(), lead)

	require.Len(t, pub.calls, 1)
	assert.Equal(t, webhooksdomain.EventLeadArchived, pub.calls[0].event)
	assert.Equal(t, userID, pub.calls[0].userID)
}

func TestWebhookPublisher_OnPendingReplyApproved(t *testing.T) {
	pub := &fakeEventPublisher{}
	b := quietWebhookPublisher(pub)
	userID := uuid.New()
	pr := &inbox.PendingReply{ID: uuid.New(), UserID: userID, LeadID: uuid.New(), Channel: inbox.ChannelEmail}

	b.OnPendingReplyApproved(context.Background(), pr)

	require.Len(t, pub.calls, 1)
	assert.Equal(t, webhooksdomain.EventPendingReplyApproved, pub.calls[0].event)
	assert.Equal(t, userID, pub.calls[0].userID)
	assert.Contains(t, string(pub.calls[0].data), pr.LeadID.String())
}

type countingQualObserver struct{ calls int }

func (c *countingQualObserver) OnLeadQualified(context.Context, *leadsdomain.Lead) { c.calls++ }

func TestCompositeQualificationObserver_FansOut(t *testing.T) {
	a, b := &countingQualObserver{}, &countingQualObserver{}
	// A nil member must be skipped without panicking.
	comp := newCompositeQualificationObserver(nil, a, nil, b)
	comp.OnLeadQualified(context.Background(), &leadsdomain.Lead{ID: uuid.New()})
	assert.Equal(t, 1, a.calls)
	assert.Equal(t, 1, b.calls)
}

type panickingQualObserver struct{}

func (panickingQualObserver) OnLeadQualified(context.Context, *leadsdomain.Lead) {
	panic("boom")
}

func TestCompositeQualificationObserver_RecoversFromPanic(t *testing.T) {
	after := &countingQualObserver{}
	comp := newCompositeQualificationObserver(slog.New(slog.NewTextHandler(io.Discard, nil)),
		panickingQualObserver{}, after)
	require.NotPanics(t, func() {
		comp.OnLeadQualified(context.Background(), &leadsdomain.Lead{ID: uuid.New()})
	})
	assert.Equal(t, 1, after.calls, "a panicking observer must not skip the rest")
}

// When webhooks are disabled (nil publisher) the bridge must be a safe no-op,
// never panicking — the observer is always wired, behavior gated by the flag.
func TestWebhookPublisher_NilPublisherIsNoOp(t *testing.T) {
	b := quietWebhookPublisher(nil)
	require.NotPanics(t, func() {
		b.OnLeadCreated(context.Background(), &inbox.InboxLead{ID: uuid.New(), UserID: uuid.New()})
		b.OnLeadQualified(context.Background(), &leadsdomain.Lead{ID: uuid.New()})
		b.OnLeadArchived(context.Background(), &leadsdomain.Lead{ID: uuid.New()})
		b.OnPendingReplyApproved(context.Background(), &inbox.PendingReply{ID: uuid.New()})
	})
}
