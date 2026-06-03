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

// fakeMapping is an in-memory MappingStore.
type fakeMapping struct {
	cfg *domain.MappingConfig
	err error
}

func (f *fakeMapping) GetActiveMappingConfig(_ context.Context, _ uuid.UUID) (*domain.MappingConfig, error) {
	return f.cfg, f.err
}

// fakeApplier records the action method invoked and its args.
type fakeApplier struct {
	action  string
	email   string
	name    string
	company string
	err     error
}

func (f *fakeApplier) HandlePayment(_ context.Context, _ uuid.UUID, email string) error {
	f.action, f.email = "payment", email
	return f.err
}
func (f *fakeApplier) HandleCounterpartyCreated(_ context.Context, _ uuid.UUID, email, name, company string) error {
	f.action, f.email, f.name, f.company = "counterparty", email, name, company
	return f.err
}
func (f *fakeApplier) HandleOrderStatus(_ context.Context, _ uuid.UUID, email string) error {
	f.action, f.email = "order_status", email
	return f.err
}
func (f *fakeApplier) HandleShipment(_ context.Context, _ uuid.UUID, email string) error {
	f.action, f.email = "shipment", email
	return f.err
}

func raw(kind string) onec.RawInboundEvent {
	return onec.RawInboundEvent{
		ExternalID:   "ОП-1",
		ExternalType: "Документ.Оплата",
		Kind:         kind,
		Payload:      []byte(`{"counterparty_email":"a@b.ru","counterparty_name":"ООО Тест","counterparty_company":"Тест"}`),
	}
}

func activeMapping(t *testing.T, extType string, kind domain.EventKind) *fakeMapping {
	t.Helper()
	cfg, err := domain.NewMappingConfig(uuid.New(), []domain.MappingRule{{
		ExternalType: extType, Kind: kind,
		EmailField: "counterparty_email", NameField: "counterparty_name", CompanyField: "counterparty_company",
	}})
	if err != nil {
		t.Fatal(err)
	}
	return &fakeMapping{cfg: cfg}
}

func TestProcessInbound_RecordOnly_NoMapping(t *testing.T) {
	store := &fakeStore{inserted: true}
	uc := onec.NewUseCase(store) // no mapping, no applier

	res, err := uc.ProcessInboundEvent(context.Background(), uuid.New(), raw("payment"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Deduped {
		t.Error("fresh event must not be deduped")
	}
	if store.calls != 1 || store.last.Kind != domain.EventKindPayment {
		t.Errorf("expected one payment record, got calls=%d kind=%v", store.calls, store.last.Kind)
	}
}

func TestProcessInbound_InvalidRequestKind(t *testing.T) {
	store := &fakeStore{inserted: true}
	uc := onec.NewUseCase(store)

	_, err := uc.ProcessInboundEvent(context.Background(), uuid.New(), raw("nonsense"))
	if !errors.Is(err, domain.ErrInvalidEventKind) {
		t.Fatalf("err = %v, want ErrInvalidEventKind", err)
	}
	if store.calls != 0 {
		t.Error("invalid kind must not record")
	}
}

func TestProcessInbound_HybridDerivesKindFromMapping(t *testing.T) {
	store := &fakeStore{inserted: true}
	mapping := activeMapping(t, "Документ.Оплата", domain.EventKindPayment)
	applier := &fakeApplier{}
	uc := onec.NewUseCase(store, onec.WithMapping(mapping), onec.WithApplier(applier))

	// empty kind → derived from mapping rule
	res, err := uc.ProcessInboundEvent(context.Background(), uuid.New(), raw(""))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Deduped {
		t.Error("fresh event not deduped")
	}
	if store.last.Kind != domain.EventKindPayment {
		t.Errorf("kind = %v, want payment (from mapping)", store.last.Kind)
	}
}

func TestProcessInbound_UnresolvableKind(t *testing.T) {
	store := &fakeStore{inserted: true}
	mapping := &fakeMapping{err: onec.ErrMappingNotFound} // no config
	uc := onec.NewUseCase(store, onec.WithMapping(mapping))

	_, err := uc.ProcessInboundEvent(context.Background(), uuid.New(), raw("")) // no kind, no mapping
	if !errors.Is(err, onec.ErrUnresolvableKind) {
		t.Fatalf("err = %v, want ErrUnresolvableKind", err)
	}
	if store.calls != 0 {
		t.Error("unresolvable event must not record")
	}
}

func TestProcessInbound_TransientMappingErrorPropagates(t *testing.T) {
	store := &fakeStore{inserted: true}
	boom := errors.New("db pool exhausted")
	mapping := &fakeMapping{err: boom}
	uc := onec.NewUseCase(store, onec.WithMapping(mapping))

	// No explicit kind → kind resolution needs the mapping; a transient error
	// must propagate (→ 500, retryable), NOT collapse into ErrUnresolvableKind.
	_, err := uc.ProcessInboundEvent(context.Background(), uuid.New(), raw(""))
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want transient boom propagated", err)
	}
	if store.calls != 0 {
		t.Error("must not record when kind unresolved due to transient error")
	}
}

func TestProcessInbound_ExplicitKindToleratesMappingError(t *testing.T) {
	store := &fakeStore{inserted: true}
	mapping := &fakeMapping{err: errors.New("boom")}
	applier := &fakeApplier{}
	uc := onec.NewUseCase(store, onec.WithMapping(mapping), onec.WithApplier(applier))

	// Explicit kind makes the event classifiable regardless of mapping health;
	// it is recorded, application is skipped (no rule available).
	res, err := uc.ProcessInboundEvent(context.Background(), uuid.New(), raw("payment"))
	if err != nil {
		t.Fatalf("explicit kind must tolerate mapping error: %v", err)
	}
	if store.calls != 1 || res.Deduped {
		t.Error("event should be recorded despite mapping error")
	}
	if applier.action != "" {
		t.Error("no rule available → application must be skipped")
	}
}

func TestProcessInbound_Routing(t *testing.T) {
	tests := []struct {
		extType    string
		kind       domain.EventKind
		wantAction string
	}{
		{"Документ.Оплата", domain.EventKindPayment, "payment"},
		{"Справочник.Контрагенты", domain.EventKindCounterpartyCreated, "counterparty"},
		{"Документ.Заказ", domain.EventKindOrderStatus, "order_status"},
		{"Документ.Отгрузка", domain.EventKindShipment, "shipment"},
	}
	for _, tt := range tests {
		t.Run(tt.wantAction, func(t *testing.T) {
			store := &fakeStore{inserted: true}
			mapping := activeMapping(t, tt.extType, tt.kind)
			applier := &fakeApplier{}
			uc := onec.NewUseCase(store, onec.WithMapping(mapping), onec.WithApplier(applier))

			ev := onec.RawInboundEvent{
				ExternalID: "X-1", ExternalType: tt.extType, Kind: "",
				Payload: []byte(`{"counterparty_email":"a@b.ru","counterparty_name":"N","counterparty_company":"C"}`),
			}
			_, err := uc.ProcessInboundEvent(context.Background(), uuid.New(), ev)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if applier.action != tt.wantAction {
				t.Fatalf("action = %q, want %q", applier.action, tt.wantAction)
			}
			if applier.email != "a@b.ru" {
				t.Errorf("email = %q, want extracted a@b.ru", applier.email)
			}
		})
	}
}

func TestProcessInbound_DedupSkipsApply(t *testing.T) {
	store := &fakeStore{inserted: false} // dedup hit
	mapping := activeMapping(t, "Документ.Оплата", domain.EventKindPayment)
	applier := &fakeApplier{}
	uc := onec.NewUseCase(store, onec.WithMapping(mapping), onec.WithApplier(applier))

	res, err := uc.ProcessInboundEvent(context.Background(), uuid.New(), raw(""))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !res.Deduped {
		t.Error("expected deduped")
	}
	if applier.action != "" {
		t.Errorf("deduped replay must not re-apply, got %q", applier.action)
	}
}

func TestProcessInbound_ApplierErrorNotFatal(t *testing.T) {
	store := &fakeStore{inserted: true}
	mapping := activeMapping(t, "Документ.Оплата", domain.EventKindPayment)
	applier := &fakeApplier{err: errors.New("downstream boom")}
	uc := onec.NewUseCase(store, onec.WithMapping(mapping), onec.WithApplier(applier))

	// applier failure is logged, not propagated — the event was recorded and a
	// 1C retry would only duplicate. Reconciliation (#109) handles re-apply.
	res, err := uc.ProcessInboundEvent(context.Background(), uuid.New(), raw(""))
	if err != nil {
		t.Fatalf("applier error must not fail the webhook: %v", err)
	}
	if res.Deduped {
		t.Error("event was still recorded")
	}
}

func TestProcessInbound_NilUser(t *testing.T) {
	store := &fakeStore{inserted: true}
	uc := onec.NewUseCase(store)

	_, err := uc.ProcessInboundEvent(context.Background(), uuid.Nil, raw("payment"))
	if !errors.Is(err, domain.ErrNilUser) {
		t.Fatalf("err = %v, want ErrNilUser", err)
	}
}
