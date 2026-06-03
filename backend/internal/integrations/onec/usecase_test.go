package onec_test

import (
	"context"
	"errors"
	"testing"

	"github.com/daniil/floq/internal/integrations/onec"
	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/google/uuid"
)

// fakeStore is an in-memory SyncStore for unit tests.
type fakeStore struct {
	inserted bool  // InsertOutcome.Inserted to return
	drifted  bool  // InsertOutcome.PayloadDrifted to return
	err      error // error to return
	calls    int   // how many times InsertSyncRecord was called
	last     *domain.SyncRecord
}

func (f *fakeStore) InsertSyncRecord(_ context.Context, rec *domain.SyncRecord) (onec.InsertOutcome, error) {
	f.calls++
	f.last = rec
	return onec.InsertOutcome{Inserted: f.inserted, PayloadDrifted: f.drifted}, f.err
}

func mustEvent(t *testing.T) *domain.ExternalEvent {
	t.Helper()
	ev, err := domain.NewExternalEvent("ОП-1", "Документ.Оплата", domain.EventKindPayment, []byte(`{"sum":1}`))
	if err != nil {
		t.Fatal(err)
	}
	return ev
}

func TestProcessInboundEvent_New(t *testing.T) {
	store := &fakeStore{inserted: true}
	uc := onec.NewUseCase(store)

	res, err := uc.ProcessInboundEvent(context.Background(), uuid.New(), mustEvent(t))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Deduped {
		t.Error("fresh event must not be marked deduped")
	}
	if store.calls != 1 {
		t.Errorf("store called %d times, want 1", store.calls)
	}
	if store.last == nil || store.last.Direction != domain.DirectionInbound {
		t.Error("expected an inbound sync record to be stored")
	}
}

func TestProcessInboundEvent_Duplicate(t *testing.T) {
	store := &fakeStore{inserted: false} // dedup hit
	uc := onec.NewUseCase(store)

	res, err := uc.ProcessInboundEvent(context.Background(), uuid.New(), mustEvent(t))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !res.Deduped {
		t.Error("replayed event must be marked deduped")
	}
}

func TestProcessInboundEvent_PayloadDrift(t *testing.T) {
	store := &fakeStore{inserted: false, drifted: true} // deduped but content changed
	uc := onec.NewUseCase(store)

	res, err := uc.ProcessInboundEvent(context.Background(), uuid.New(), mustEvent(t))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !res.Deduped {
		t.Error("drifted replay is still deduped")
	}
	if !res.PayloadDrifted {
		t.Error("drift must be surfaced in the result")
	}
}

func TestProcessInboundEvent_NilUser(t *testing.T) {
	store := &fakeStore{inserted: true}
	uc := onec.NewUseCase(store)

	_, err := uc.ProcessInboundEvent(context.Background(), uuid.Nil, mustEvent(t))
	if !errors.Is(err, domain.ErrNilUser) {
		t.Fatalf("err = %v, want ErrNilUser", err)
	}
	if store.calls != 0 {
		t.Error("store must not be called when record construction fails")
	}
}

func TestProcessInboundEvent_NilEvent(t *testing.T) {
	store := &fakeStore{inserted: true}
	uc := onec.NewUseCase(store)

	_, err := uc.ProcessInboundEvent(context.Background(), uuid.New(), nil)
	if !errors.Is(err, domain.ErrNilEvent) {
		t.Fatalf("err = %v, want ErrNilEvent", err)
	}
}

func TestProcessInboundEvent_StoreError(t *testing.T) {
	sentinel := errors.New("db down")
	store := &fakeStore{err: sentinel}
	uc := onec.NewUseCase(store)

	_, err := uc.ProcessInboundEvent(context.Background(), uuid.New(), mustEvent(t))
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel", err)
	}
}
