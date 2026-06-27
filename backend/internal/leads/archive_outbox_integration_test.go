//go:build integration

package leads_test

import (
	"context"
	"errors"
	"testing"

	"github.com/daniil/floq/internal/db"
	"github.com/daniil/floq/internal/leads"
	"github.com/daniil/floq/internal/leads/domain"
	"github.com/daniil/floq/internal/testutil"
	"github.com/daniil/floq/internal/webhooks"
	webhooksdomain "github.com/daniil/floq/internal/webhooks/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// archiveOutboxEmitter enqueues a lead.archived delivery via the REAL webhooks
// repository, so the delivery shares ArchiveLead's transaction — the #199
// transactional-outbox contract. It always enqueues first; when fail is set it
// then returns an error, forcing the transaction to roll back BOTH the archive
// write and the just-enqueued delivery.
type archiveOutboxEmitter struct {
	repo       *webhooks.Repository
	userID     uuid.UUID
	endpointID uuid.UUID
	fail       bool
}

func (e *archiveOutboxEmitter) EmitLeadArchived(ctx context.Context, lead *domain.Lead) error {
	d, err := webhooksdomain.NewDelivery(e.userID, e.endpointID, webhooksdomain.EventLeadArchived,
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

// End-to-end proof of the #199 DoD: ArchiveLead's domain write and the webhook
// delivery enqueue are atomic. On success exactly one delivery is committed; on
// a failing emit neither the archive nor the delivery survives.
func TestArchiveLead_TransactionalOutbox_CommitAndRollback(t *testing.T) {
	pool := testutil.TestDB(t)
	userID := testutil.SeedUser(t, pool)
	leadsRepo := leads.NewRepository(pool)
	whRepo := webhooks.NewRepository(pool, testutil.NewSecretCipher(t))
	tm := db.NewTxManager(pool)
	ctx := context.Background()

	// A subscribed endpoint — the FK target for delivery rows.
	ep, err := webhooksdomain.NewWebhookEndpoint(userID, "https://example.com/hook",
		[]webhooksdomain.EventType{webhooksdomain.EventLeadArchived}, "supersecretvalue123")
	require.NoError(t, err)
	require.NoError(t, whRepo.CreateEndpoint(ctx, ep))

	countDeliveries := func() int {
		var n int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*) FROM webhook_deliveries WHERE user_id = $1`, userID).Scan(&n))
		return n
	}

	// --- Commit path: archive + delivery commit together. ---
	committed := seedLead(t, leadsRepo, userID, domain.StatusNew)
	uc := leads.NewUseCase(leadsRepo, nil, nil,
		leads.WithTxManager(tm),
		leads.WithLeadArchivedEmitter(&archiveOutboxEmitter{repo: whRepo, userID: userID, endpointID: ep.ID}))
	require.NoError(t, uc.ArchiveLead(ctx, committed.ID))

	got, err := leadsRepo.GetLead(ctx, committed.ID)
	require.NoError(t, err)
	require.NotNil(t, got.ArchivedAt, "lead must be archived on the commit path")
	require.Equal(t, 1, countDeliveries(), "exactly one delivery must be enqueued in the same transaction")

	// --- Rollback path: a failing emit aborts the archive AND the delivery. ---
	rolled := seedLead(t, leadsRepo, userID, domain.StatusNew)
	ucFail := leads.NewUseCase(leadsRepo, nil, nil,
		leads.WithTxManager(tm),
		leads.WithLeadArchivedEmitter(&archiveOutboxEmitter{repo: whRepo, userID: userID, endpointID: ep.ID, fail: true}))
	require.Error(t, ucFail.ArchiveLead(ctx, rolled.ID), "a failed emit must abort the archive")

	got, err = leadsRepo.GetLead(ctx, rolled.ID)
	require.NoError(t, err)
	assert.Nil(t, got.ArchivedAt, "a rolled-back archive must leave the lead un-archived")
	assert.Equal(t, 1, countDeliveries(),
		"no delivery may survive the rolled-back transaction (still just the committed one)")
}
