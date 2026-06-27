//go:build integration

package inbox

import (
	"context"
	"errors"
	"testing"

	"github.com/daniil/floq/internal/db"
	"github.com/daniil/floq/internal/leads"
	leadsdomain "github.com/daniil/floq/internal/leads/domain"
	"github.com/daniil/floq/internal/testutil"
	"github.com/daniil/floq/internal/webhooks"
	webhooksdomain "github.com/daniil/floq/internal/webhooks/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// leadsRepoForIntake satisfies inbox.LeadRepository but only implements the one
// method commitLeadIntake exercises (CreateLead), backed by the REAL leads
// repository so the insert participates in the ambient transaction via
// ConnFromCtx. Any other method call would be a test bug — the embedded nil
// interface panics, surfacing it loudly.
type leadsRepoForIntake struct {
	LeadRepository
	real *leads.Repository
}

func (a *leadsRepoForIntake) CreateLead(ctx context.Context, l *InboxLead) error {
	return a.real.CreateLead(ctx, &leadsdomain.Lead{
		ID:             l.ID,
		UserID:         l.UserID,
		Channel:        leadsdomain.Channel(l.Channel),
		ContactName:    l.ContactName,
		Company:        l.Company,
		FirstMessage:   l.FirstMessage,
		Status:         leadsdomain.LeadStatus(l.Status),
		EmailAddress:   l.EmailAddress,
		TelegramChatID: l.TelegramChatID,
		CreatedAt:      l.CreatedAt,
		UpdatedAt:      l.UpdatedAt,
	})
}

// createdOutboxEmitter enqueues a lead.created delivery through the REAL webhooks
// repository so it joins commitLeadIntake's transaction. When fail is set it
// returns an error after enqueueing, forcing the whole transaction — both the
// lead row and the delivery — to roll back.
type createdOutboxEmitter struct {
	repo       *webhooks.Repository
	userID     uuid.UUID
	endpointID uuid.UUID
	fail       bool
}

func (e *createdOutboxEmitter) EmitLeadCreated(ctx context.Context, lead *InboxLead) error {
	d, err := webhooksdomain.NewDelivery(e.userID, e.endpointID, webhooksdomain.EventLeadCreated,
		[]byte(`{"id":"`+lead.ID.String()+`"}`))
	if err != nil {
		return err
	}
	if err := e.repo.EnqueueDelivery(ctx, d); err != nil {
		return err
	}
	if e.fail {
		return errors.New("emit boom after enqueue")
	}
	return nil
}

// End-to-end proof of the #206 Part A contract: the email poller's new-lead
// intake (CreateLead) and the lead.created enqueue are atomic on the real DB. On
// success exactly one delivery commits alongside the lead; on a failing emit
// neither the lead nor the delivery survives, so the poll loop can safely leave
// the source email unseen and re-process it.
func TestEmailPoller_CommitLeadIntake_TransactionalOutbox(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	leadsRepo := leads.NewRepository(pool)
	whRepo := webhooks.NewRepository(pool, testutil.NewSecretCipher(t))
	tm := db.NewTxManager(pool)
	ctx := context.Background()

	ep, err := webhooksdomain.NewWebhookEndpoint(userID, "https://example.com/hook",
		[]webhooksdomain.EventType{webhooksdomain.EventLeadCreated}, "supersecretvalue123")
	require.NoError(t, err)
	require.NoError(t, whRepo.CreateEndpoint(ctx, ep))

	countDeliveries := func() int {
		var n int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM webhook_deliveries WHERE user_id = $1`, userID).Scan(&n))
		return n
	}
	leadExists := func(id uuid.UUID) bool {
		var n int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM leads WHERE id = $1`, id).Scan(&n))
		return n == 1
	}

	newPoller := func(fail bool) (*EmailPoller, *InboxLead) {
		addr := "lead@example.com"
		lead, lerr := NewInboxLead(userID, ChannelEmail, "Lead", "", "I need a website", nil, &addr)
		require.NoError(t, lerr)
		// Distinct IDs per case so the two rows never collide.
		lead.ID = uuid.New()
		p := &EmailPoller{
			repo:               &leadsRepoForIntake{real: leadsRepo},
			ownerID:            userID,
			tx:                 tm,
			leadCreatedEmitter: &createdOutboxEmitter{repo: whRepo, userID: userID, endpointID: ep.ID, fail: fail},
		}
		return p, lead
	}

	// --- Commit path: lead + delivery commit together. ---
	pCommit, committed := newPoller(false)
	require.NoError(t, pCommit.commitLeadIntake(ctx, committed))
	require.True(t, leadExists(committed.ID), "lead must persist on the commit path")
	require.Equal(t, 1, countDeliveries(), "exactly one delivery must be enqueued in the same transaction")

	// --- Rollback path: a failing emit aborts the lead AND the delivery. ---
	pFail, rolled := newPoller(true)
	require.Error(t, pFail.commitLeadIntake(ctx, rolled), "a failed emit must abort the intake")
	assert.False(t, leadExists(rolled.ID), "a rolled-back intake must leave no lead row")
	assert.Equal(t, 1, countDeliveries(),
		"no delivery may survive the rolled-back transaction (still just the committed one)")
}
