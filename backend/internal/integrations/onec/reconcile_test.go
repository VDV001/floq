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

type fakeReconcileStore struct {
	activeIDs []uuid.UUID
	activeErr error
	creds     *domain.OutboundCredentials
	credsErr  error
}

func (f *fakeReconcileStore) ActiveOnecUserIDs(_ context.Context) ([]uuid.UUID, error) {
	return f.activeIDs, f.activeErr
}
func (f *fakeReconcileStore) GetOutboundCredentials(_ context.Context, _ uuid.UUID) (*domain.OutboundCredentials, error) {
	return f.creds, f.credsErr
}

type fakeReader struct {
	events []RawInboundEvent
	err    error
	calls  int
}

func (f *fakeReader) ListEvents(_ context.Context, _ *domain.OutboundCredentials) ([]RawInboundEvent, error) {
	f.calls++
	return f.events, f.err
}

type fakeProcessor struct {
	calls int
	fn    func(RawInboundEvent) (ProcessResult, error)
}

func (f *fakeProcessor) ProcessInboundEvent(_ context.Context, _ uuid.UUID, in RawInboundEvent) (ProcessResult, error) {
	f.calls++
	return f.fn(in)
}

func reconcileCreds(t *testing.T) *domain.OutboundCredentials {
	t.Helper()
	c, err := domain.NewOutboundCredentials("https://1c.example", domain.AuthTypeBasic, "s")
	require.NoError(t, err)
	return c
}

func TestReconcileUser_AppliesMissedEvent(t *testing.T) {
	store := &fakeReconcileStore{creds: reconcileCreds(t)}
	reader := &fakeReader{events: []RawInboundEvent{{ExternalID: "doc-1", ExternalType: "X", Kind: "payment"}}}
	proc := &fakeProcessor{fn: func(RawInboundEvent) (ProcessResult, error) {
		return ProcessResult{Deduped: false}, nil // was missing → freshly applied
	}}
	uc := NewReconcileUseCase(store, reader, proc, nil)

	stats, err := uc.ReconcileUser(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.Equal(t, 1, stats.Fetched)
	assert.Equal(t, 1, stats.Applied, "a missed event must be applied")
	assert.Equal(t, 0, stats.Deduped)
}

func TestReconcileUser_SkipsAlreadyProcessed(t *testing.T) {
	store := &fakeReconcileStore{creds: reconcileCreds(t)}
	reader := &fakeReader{events: []RawInboundEvent{{ExternalID: "doc-1", ExternalType: "X", Kind: "payment"}}}
	proc := &fakeProcessor{fn: func(RawInboundEvent) (ProcessResult, error) {
		return ProcessResult{Deduped: true}, nil // already in ledger → no-op
	}}
	uc := NewReconcileUseCase(store, reader, proc, nil)

	stats, err := uc.ReconcileUser(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.Equal(t, 1, stats.Deduped)
	assert.Equal(t, 0, stats.Applied, "already-processed event must not be re-applied")
}

func TestReconcileUser_UnconfiguredSkips(t *testing.T) {
	store := &fakeReconcileStore{credsErr: ErrOutboundNotConfigured}
	reader := &fakeReader{}
	proc := &fakeProcessor{fn: func(RawInboundEvent) (ProcessResult, error) { return ProcessResult{}, nil }}
	uc := NewReconcileUseCase(store, reader, proc, nil)

	stats, err := uc.ReconcileUser(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.Equal(t, 0, reader.calls, "must not read 1C when unconfigured")
	assert.Equal(t, 0, stats.Fetched)
}

func TestReconcileUser_OneBadEventDoesNotStopBatch(t *testing.T) {
	store := &fakeReconcileStore{creds: reconcileCreds(t)}
	reader := &fakeReader{events: []RawInboundEvent{
		{ExternalID: "bad", ExternalType: "X"},
		{ExternalID: "good", ExternalType: "X", Kind: "payment"},
	}}
	proc := &fakeProcessor{fn: func(in RawInboundEvent) (ProcessResult, error) {
		if in.ExternalID == "bad" {
			return ProcessResult{}, errors.New("unresolvable")
		}
		return ProcessResult{Deduped: false}, nil
	}}
	uc := NewReconcileUseCase(store, reader, proc, nil)

	stats, err := uc.ReconcileUser(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.Equal(t, 2, stats.Fetched)
	assert.Equal(t, 1, stats.Failed)
	assert.Equal(t, 1, stats.Applied, "a later good event still applies after an earlier failure")
}

func TestReconcileAll_IteratesActiveUsers(t *testing.T) {
	store := &fakeReconcileStore{
		activeIDs: []uuid.UUID{uuid.New(), uuid.New()},
		creds:     reconcileCreds(t),
	}
	reader := &fakeReader{events: []RawInboundEvent{{ExternalID: "d", ExternalType: "X", Kind: "payment"}}}
	proc := &fakeProcessor{fn: func(RawInboundEvent) (ProcessResult, error) {
		return ProcessResult{Deduped: true}, nil
	}}
	uc := NewReconcileUseCase(store, reader, proc, nil)

	err := uc.ReconcileAll(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, reader.calls, "each active user must be reconciled")
}
