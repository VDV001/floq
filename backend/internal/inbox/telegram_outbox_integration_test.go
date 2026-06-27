//go:build integration

package inbox

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/db"
	"github.com/daniil/floq/internal/leads"
	"github.com/daniil/floq/internal/testutil"
	"github.com/daniil/floq/internal/webhooks"
	webhooksdomain "github.com/daniil/floq/internal/webhooks/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// End-to-end proof of the #206 Part B contract: the Telegram bot's new-lead
// intake (CreateLead) and the lead.created enqueue are atomic on the real DB. On
// success exactly one delivery commits alongside the lead; on a failing emit
// neither survives, so the receive loop can safely leave the update offset
// un-advanced and re-deliver it. Shares the test adapter/emitter from
// email_outbox_integration_test.go — only the lead's channel differs.
func TestTelegramBot_CommitLeadIntake_TransactionalOutbox(t *testing.T) {
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

	newBot := func(fail bool) (*TelegramBot, *InboxLead) {
		chatID := int64(987654)
		lead, lerr := NewInboxLead(userID, ChannelTelegram, "Lead", "", "I need a website", &chatID, nil)
		require.NoError(t, lerr)
		lead.ID = uuid.New()
		b := &TelegramBot{
			repo:               &leadsRepoForIntake{real: leadsRepo},
			ownerID:            userID,
			tx:                 tm,
			leadCreatedEmitter: &createdOutboxEmitter{repo: whRepo, userID: userID, endpointID: ep.ID, fail: fail},
		}
		return b, lead
	}

	// --- Commit path: lead + delivery commit together. ---
	bCommit, committed := newBot(false)
	require.NoError(t, bCommit.commitLeadIntake(ctx, committed))
	require.True(t, leadExists(committed.ID), "lead must persist on the commit path")
	require.Equal(t, 1, countDeliveries(), "exactly one delivery must be enqueued in the same transaction")

	// --- Rollback path: a failing emit aborts the lead AND the delivery. ---
	bFail, rolled := newBot(true)
	require.Error(t, bFail.commitLeadIntake(ctx, rolled), "a failed emit must abort the intake")
	assert.False(t, leadExists(rolled.ID), "a rolled-back intake must leave no lead row")
	assert.Equal(t, 1, countDeliveries(),
		"no delivery may survive the rolled-back transaction (still just the committed one)")
}
