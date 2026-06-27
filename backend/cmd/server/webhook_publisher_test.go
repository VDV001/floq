package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	inbox "github.com/daniil/floq/internal/inbox"
	leadsdomain "github.com/daniil/floq/internal/leads/domain"
	"github.com/daniil/floq/internal/outbound"
	webhooksdomain "github.com/daniil/floq/internal/webhooks/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeEventPublisher struct {
	calls []publishedEvent
	err   error
}

type publishedEvent struct {
	userID uuid.UUID
	event  webhooksdomain.EventType
	data   json.RawMessage
}

func (f *fakeEventPublisher) Publish(_ context.Context, userID uuid.UUID, event webhooksdomain.EventType, data json.RawMessage) (int, error) {
	f.calls = append(f.calls, publishedEvent{userID, event, data})
	if f.err != nil {
		return 0, f.err
	}
	return 1, nil
}

func quietWebhookPublisher(p eventPublisher) *webhookEventPublisher {
	return newWebhookEventPublisher(p, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestWebhookPublisher_EmitLeadCreated(t *testing.T) {
	pub := &fakeEventPublisher{}
	b := quietWebhookPublisher(pub)
	userID := uuid.New()
	email := "ivan@acme.ru"
	lead := &inbox.InboxLead{ID: uuid.New(), UserID: userID, Channel: inbox.ChannelTelegram,
		ContactName: "Ivan", Company: "Acme", EmailAddress: &email}

	require.NoError(t, b.EmitLeadCreated(context.Background(), lead))

	require.Len(t, pub.calls, 1)
	assert.Equal(t, webhooksdomain.EventLeadCreated, pub.calls[0].event)
	assert.Equal(t, userID, pub.calls[0].userID)
	assert.Contains(t, string(pub.calls[0].data), lead.ID.String())
	assert.Contains(t, string(pub.calls[0].data), "ivan@acme.ru")
}

func TestWebhookPublisher_EmitLeadQualified(t *testing.T) {
	pub := &fakeEventPublisher{}
	b := quietWebhookPublisher(pub)
	userID := uuid.New()
	lead := &leadsdomain.Lead{ID: uuid.New(), UserID: userID, ContactName: "Ann",
		Status: leadsdomain.StatusQualified}

	require.NoError(t, b.EmitLeadQualified(context.Background(), lead))

	require.Len(t, pub.calls, 1)
	assert.Equal(t, webhooksdomain.EventLeadQualified, pub.calls[0].event)
	assert.Equal(t, userID, pub.calls[0].userID)
	assert.Contains(t, string(pub.calls[0].data), "qualified")
}

func TestWebhookPublisher_EmitLeadArchived(t *testing.T) {
	pub := &fakeEventPublisher{}
	b := quietWebhookPublisher(pub)
	userID := uuid.New()
	lead := &leadsdomain.Lead{ID: uuid.New(), UserID: userID}

	require.NoError(t, b.EmitLeadArchived(context.Background(), lead))

	require.Len(t, pub.calls, 1)
	assert.Equal(t, webhooksdomain.EventLeadArchived, pub.calls[0].event)
	assert.Equal(t, userID, pub.calls[0].userID)
}

func TestWebhookPublisher_EmitPendingReplyApproved(t *testing.T) {
	pub := &fakeEventPublisher{}
	b := quietWebhookPublisher(pub)
	userID := uuid.New()
	pr := &inbox.PendingReply{ID: uuid.New(), UserID: userID, LeadID: uuid.New(), Channel: inbox.ChannelEmail}

	require.NoError(t, b.EmitPendingReplyApproved(context.Background(), pr))

	require.Len(t, pub.calls, 1)
	assert.Equal(t, webhooksdomain.EventPendingReplyApproved, pub.calls[0].event)
	assert.Equal(t, userID, pub.calls[0].userID)
	assert.Contains(t, string(pub.calls[0].data), pr.LeadID.String())
}

func TestWebhookPublisher_EmitSequenceCompleted(t *testing.T) {
	pub := &fakeEventPublisher{}
	b := quietWebhookPublisher(pub)
	userID := uuid.New()
	ev := outbound.SequenceCompletion{UserID: userID, ProspectID: uuid.New(), SequenceID: uuid.New()}

	require.NoError(t, b.EmitSequenceCompleted(context.Background(), ev))

	require.Len(t, pub.calls, 1)
	assert.Equal(t, webhooksdomain.EventSequenceCompleted, pub.calls[0].event)
	assert.Equal(t, userID, pub.calls[0].userID)
	assert.Contains(t, string(pub.calls[0].data), ev.SequenceID.String())
}

// A publish failure must propagate as an error so the caller's transaction rolls
// back (#199 fail-closed) — unlike the old observer that swallowed it.
func TestWebhookPublisher_EmitPropagatesPublishError(t *testing.T) {
	pub := &fakeEventPublisher{err: errors.New("endpoint store down")}
	b := quietWebhookPublisher(pub)

	err := b.EmitLeadArchived(context.Background(), &leadsdomain.Lead{ID: uuid.New(), UserID: uuid.New()})
	require.Error(t, err, "an enqueue failure must surface so the domain transaction can roll back")
}

// When webhooks are disabled (nil publisher) the bridge must be a safe no-op,
// returning nil so a usecase that wires it unconditionally stays unaffected.
func TestWebhookPublisher_NilPublisherIsNoOp(t *testing.T) {
	b := quietWebhookPublisher(nil)
	require.NotPanics(t, func() {
		assert.NoError(t, b.EmitLeadCreated(context.Background(), &inbox.InboxLead{ID: uuid.New(), UserID: uuid.New()}))
		assert.NoError(t, b.EmitLeadQualified(context.Background(), &leadsdomain.Lead{ID: uuid.New()}))
		assert.NoError(t, b.EmitLeadArchived(context.Background(), &leadsdomain.Lead{ID: uuid.New()}))
		assert.NoError(t, b.EmitPendingReplyApproved(context.Background(), &inbox.PendingReply{ID: uuid.New()}))
		assert.NoError(t, b.EmitSequenceCompleted(context.Background(), outbound.SequenceCompletion{}))
	})
}
