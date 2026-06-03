package onec

import (
	"context"
	"errors"
	"testing"

	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeOutboundStore struct {
	creds        *domain.OutboundCredentials
	credsErr     error
	processed    bool
	processedErr error
	upserted     []*domain.SyncRecord
	upsertErr    error
}

func (f *fakeOutboundStore) GetOutboundCredentials(_ context.Context, _ uuid.UUID) (*domain.OutboundCredentials, error) {
	return f.creds, f.credsErr
}
func (f *fakeOutboundStore) OutboundProcessedExists(_ context.Context, _ uuid.UUID, _, _ string) (bool, error) {
	return f.processed, f.processedErr
}
func (f *fakeOutboundStore) UpsertOutboundRecord(_ context.Context, rec *domain.SyncRecord) error {
	f.upserted = append(f.upserted, rec)
	return f.upsertErr
}

type fakeClient struct {
	ref   string
	err   error
	calls int
}

func (f *fakeClient) CreateCounterparty(_ context.Context, _ *domain.OutboundCredentials, _ *domain.CounterpartyDraft) (string, error) {
	f.calls++
	return f.ref, f.err
}

func okCreds(t *testing.T) *domain.OutboundCredentials {
	t.Helper()
	c, err := domain.NewOutboundCredentials("https://1c.example", domain.AuthTypeBasic, "s")
	require.NoError(t, err)
	return c
}

func draftWithEmail(t *testing.T) *domain.CounterpartyDraft {
	t.Helper()
	d, err := domain.NewCounterpartyDraft("Иван", "iv@ex.ru", "ООО Ромашка")
	require.NoError(t, err)
	return d
}

func TestPushCounterparty_Success(t *testing.T) {
	store := &fakeOutboundStore{creds: okCreds(t)}
	client := &fakeClient{ref: "ctr-1"}
	uc := NewOutboundUseCase(store, client, nil)

	err := uc.PushCounterparty(context.Background(), uuid.New(), draftWithEmail(t))

	require.NoError(t, err)
	assert.Equal(t, 1, client.calls)
	require.Len(t, store.upserted, 1)
	rec := store.upserted[0]
	assert.Equal(t, domain.SyncStatusProcessed, rec.Status)
	assert.Equal(t, domain.DirectionOutbound, rec.Direction)
	assert.Equal(t, "iv@ex.ru", rec.ExternalID, "email is the dedup identity")
	assert.Equal(t, "counterparty", rec.ExternalType)
	assert.Equal(t, domain.EventKindCounterpartyCreated, rec.Kind)
}

func TestPushCounterparty_NotConfigured_NoOp(t *testing.T) {
	store := &fakeOutboundStore{credsErr: ErrOutboundNotConfigured}
	client := &fakeClient{}
	uc := NewOutboundUseCase(store, client, nil)

	err := uc.PushCounterparty(context.Background(), uuid.New(), draftWithEmail(t))

	require.NoError(t, err, "no 1C endpoint configured is a silent no-op, not an error")
	assert.Equal(t, 0, client.calls, "must not call 1C when unconfigured")
	assert.Empty(t, store.upserted, "must not write a ledger record when unconfigured")
}

func TestPushCounterparty_AlreadyProcessed_Dedup(t *testing.T) {
	store := &fakeOutboundStore{creds: okCreds(t), processed: true}
	client := &fakeClient{}
	uc := NewOutboundUseCase(store, client, nil)

	err := uc.PushCounterparty(context.Background(), uuid.New(), draftWithEmail(t))

	require.NoError(t, err)
	assert.Equal(t, 0, client.calls, "already-processed counterparty must not be re-created")
}

func TestPushCounterparty_OneCError_RecordedNotFatal(t *testing.T) {
	store := &fakeOutboundStore{creds: okCreds(t)}
	client := &fakeClient{err: errors.New("1C down")}
	uc := NewOutboundUseCase(store, client, nil)

	err := uc.PushCounterparty(context.Background(), uuid.New(), draftWithEmail(t))

	require.Error(t, err, "the failure is surfaced for observability")
	require.Len(t, store.upserted, 1, "the failed push must still land in the ledger")
	assert.Equal(t, domain.SyncStatusError, store.upserted[0].Status)
}

func TestPushCounterparty_CredsLookupError_Propagates(t *testing.T) {
	store := &fakeOutboundStore{credsErr: errors.New("db boom")}
	client := &fakeClient{}
	uc := NewOutboundUseCase(store, client, nil)

	err := uc.PushCounterparty(context.Background(), uuid.New(), draftWithEmail(t))

	require.Error(t, err, "a transient creds lookup failure is not the same as 'not configured'")
	assert.Equal(t, 0, client.calls)
}
