package main

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	onecdomain "github.com/daniil/floq/internal/integrations/onec/domain"
	leadsdomain "github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCounterpartyPusher struct {
	calls     int32
	done      chan struct{}
	gotUserID uuid.UUID
	gotDraft  *onecdomain.CounterpartyDraft
}

func (f *fakeCounterpartyPusher) PushCounterparty(_ context.Context, userID uuid.UUID, draft *onecdomain.CounterpartyDraft) error {
	atomic.AddInt32(&f.calls, 1)
	f.gotUserID = userID
	f.gotDraft = draft
	if f.done != nil {
		close(f.done)
	}
	return nil
}

func quietAdapter(p counterpartyPusher) *onecQualificationAdapter {
	return newOnecQualificationAdapter(p, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestOnecQualificationAdapter_SkipsWhenNoNameOrEmail(t *testing.T) {
	pusher := &fakeCounterpartyPusher{}
	adapter := quietAdapter(pusher)

	// No name, no email → cannot form a counterparty draft. The skip branch is
	// synchronous (no goroutine spawned), so the call count is settled on return.
	adapter.OnLeadQualified(context.Background(), &leadsdomain.Lead{
		ID:     uuid.New(),
		UserID: uuid.New(),
	})

	assert.Equal(t, int32(0), atomic.LoadInt32(&pusher.calls), "no name/email must not push")
}

func TestOnecQualificationAdapter_PushesQualifiedLead(t *testing.T) {
	email := "iv@ex.ru"
	userID := uuid.New()
	pusher := &fakeCounterpartyPusher{done: make(chan struct{})}
	adapter := quietAdapter(pusher)

	adapter.OnLeadQualified(context.Background(), &leadsdomain.Lead{
		ID:           uuid.New(),
		UserID:       userID,
		ContactName:  "Иван",
		Company:      "ООО Ромашка",
		EmailAddress: &email,
	})

	// The push is detached into a goroutine — wait for it (bounded).
	select {
	case <-pusher.done:
	case <-time.After(2 * time.Second):
		t.Fatal("push goroutine did not run within 2s")
	}

	assert.Equal(t, int32(1), atomic.LoadInt32(&pusher.calls))
	assert.Equal(t, userID, pusher.gotUserID)
	require.NotNil(t, pusher.gotDraft)
	assert.Equal(t, "Иван", pusher.gotDraft.Name)
	assert.Equal(t, "iv@ex.ru", pusher.gotDraft.Email)
	assert.Equal(t, "ООО Ромашка", pusher.gotDraft.Company)
}
