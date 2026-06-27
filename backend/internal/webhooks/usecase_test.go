package webhooks

import (
	"context"
	"errors"
	"testing"

	"github.com/daniil/floq/internal/webhooks/domain"
	"github.com/google/uuid"
)

// --- fakes ---

type activeUpdate struct {
	id     uuid.UUID
	active bool
}

type fakeStore struct {
	endpoints     map[uuid.UUID]*domain.WebhookEndpoint
	deliveries    []*domain.WebhookDelivery
	activeUpdates []activeUpdate
	saveErr       error
}

func newFakeStore() *fakeStore {
	return &fakeStore{endpoints: map[uuid.UUID]*domain.WebhookEndpoint{}}
}

func (f *fakeStore) CreateEndpoint(_ context.Context, e *domain.WebhookEndpoint) error {
	f.endpoints[e.ID] = e
	return nil
}
func (f *fakeStore) ListEndpoints(_ context.Context, userID uuid.UUID) ([]*domain.WebhookEndpoint, error) {
	var out []*domain.WebhookEndpoint
	for _, e := range f.endpoints {
		if e.UserID == userID {
			out = append(out, e)
		}
	}
	return out, nil
}
func (f *fakeStore) GetEndpoint(_ context.Context, id uuid.UUID) (*domain.WebhookEndpoint, bool, error) {
	e, ok := f.endpoints[id]
	return e, ok, nil
}
func (f *fakeStore) DeleteEndpoint(_ context.Context, id uuid.UUID) error {
	delete(f.endpoints, id)
	return nil
}
func (f *fakeStore) SetEndpointActive(_ context.Context, id uuid.UUID, active bool) error {
	f.activeUpdates = append(f.activeUpdates, activeUpdate{id: id, active: active})
	if e, ok := f.endpoints[id]; ok {
		e.Active = active
	}
	return nil
}
func (f *fakeStore) EnqueueDelivery(_ context.Context, d *domain.WebhookDelivery) error {
	f.deliveries = append(f.deliveries, d)
	return nil
}
func (f *fakeStore) ClaimDueDeliveries(_ context.Context, limit, maxAttempts int) ([]*domain.WebhookDelivery, error) {
	var out []*domain.WebhookDelivery
	for _, d := range f.deliveries {
		if d.Status == domain.DeliveryPending && d.Attempts < maxAttempts && len(out) < limit {
			out = append(out, d)
		}
	}
	return out, nil
}
func (f *fakeStore) SaveDelivery(_ context.Context, _ *domain.WebhookDelivery) error {
	return f.saveErr
}

type fakeClient struct {
	status     int
	err        error
	gotURL     string
	gotSig     string
	gotBody    []byte
	gotEventID string
	calls      int
}

func (c *fakeClient) Deliver(_ context.Context, url string, payload []byte, sig, eventID string) (int, error) {
	c.calls++
	c.gotURL, c.gotBody, c.gotSig, c.gotEventID = url, payload, sig, eventID
	return c.status, c.err
}

type fakeObserver struct {
	events  []string
	success []bool
}

func (o *fakeObserver) OnWebhookDelivery(eventType string, success bool) {
	o.events = append(o.events, eventType)
	o.success = append(o.success, success)
}

func cfg() Config { return Config{MaxAttempts: 3, BatchLimit: 10} }

func mustEndpoint(t *testing.T, userID uuid.UUID, events ...domain.EventType) *domain.WebhookEndpoint {
	t.Helper()
	ep, err := domain.NewWebhookEndpoint(userID, "https://example.com/hook", events, "supersecretvalue123")
	if err != nil {
		t.Fatalf("build endpoint: %v", err)
	}
	return ep
}

// --- tests ---

func TestCreateEndpoint_ValidatesAndStores(t *testing.T) {
	store := newFakeStore()
	uc := NewUseCase(store, &fakeClient{}, cfg(), nil)
	userID := uuid.New()

	ep, err := uc.CreateEndpoint(context.Background(), userID, "https://example.com/h",
		[]string{"lead.created", "lead.qualified"}, "supersecretvalue123")
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}
	if ep.UserID != userID || len(store.endpoints) != 1 {
		t.Fatal("endpoint not stored under owner")
	}
}

func TestCreateEndpoint_RejectsBadInput(t *testing.T) {
	uc := NewUseCase(newFakeStore(), &fakeClient{}, cfg(), nil)
	// SSRF URL rejected by the domain VO, surfaced by the usecase.
	if _, err := uc.CreateEndpoint(context.Background(), uuid.New(), "http://127.0.0.1/x",
		[]string{"lead.created"}, "supersecretvalue123"); !errors.Is(err, domain.ErrInvalidWebhookURL) {
		t.Fatalf("want ErrInvalidWebhookURL, got %v", err)
	}
	// Unknown event string rejected.
	if _, err := uc.CreateEndpoint(context.Background(), uuid.New(), "https://x.com/h",
		[]string{"lead.exploded"}, "supersecretvalue123"); !errors.Is(err, domain.ErrUnknownEventType) {
		t.Fatalf("want ErrUnknownEventType, got %v", err)
	}
}

func TestDeleteEndpoint_OwnershipEnforced(t *testing.T) {
	store := newFakeStore()
	owner := uuid.New()
	ep := mustEndpoint(t, owner, domain.EventLeadCreated)
	store.endpoints[ep.ID] = ep
	uc := NewUseCase(store, &fakeClient{}, cfg(), nil)

	// A different user cannot delete it.
	if err := uc.DeleteEndpoint(context.Background(), uuid.New(), ep.ID); !errors.Is(err, ErrEndpointNotFound) {
		t.Fatalf("cross-tenant delete: want ErrEndpointNotFound, got %v", err)
	}
	if _, ok := store.endpoints[ep.ID]; !ok {
		t.Fatal("endpoint must survive a non-owner delete")
	}
	// The owner can.
	if err := uc.DeleteEndpoint(context.Background(), owner, ep.ID); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
	if _, ok := store.endpoints[ep.ID]; ok {
		t.Fatal("endpoint must be gone after owner delete")
	}
}

func TestSetEndpointActive_TogglesPersistsAndReturns(t *testing.T) {
	store := newFakeStore()
	uc := NewUseCase(store, &fakeClient{}, cfg(), nil)
	owner := uuid.New()
	ep := mustEndpoint(t, owner, domain.EventLeadCreated)
	store.endpoints[ep.ID] = ep

	got, err := uc.SetEndpointActive(context.Background(), owner, ep.ID, false)
	if err != nil {
		t.Fatalf("SetEndpointActive(false): %v", err)
	}
	if got.Active {
		t.Fatal("returned endpoint must be inactive")
	}
	// The new state must be persisted through the store, with the right args.
	if len(store.activeUpdates) != 1 || store.activeUpdates[0].id != ep.ID || store.activeUpdates[0].active {
		t.Fatalf("expected one persist of (id=%s, active=false), got %+v", ep.ID, store.activeUpdates)
	}

	got, err = uc.SetEndpointActive(context.Background(), owner, ep.ID, true)
	if err != nil {
		t.Fatalf("SetEndpointActive(true): %v", err)
	}
	if !got.Active {
		t.Fatal("returned endpoint must be active again")
	}
	if len(store.activeUpdates) != 2 || !store.activeUpdates[1].active {
		t.Fatalf("expected reactivation persisted, got %+v", store.activeUpdates)
	}
}

func TestSetEndpointActive_OwnershipEnforced(t *testing.T) {
	store := newFakeStore()
	uc := NewUseCase(store, &fakeClient{}, cfg(), nil)
	owner := uuid.New()
	ep := mustEndpoint(t, owner, domain.EventLeadCreated)
	store.endpoints[ep.ID] = ep

	_, err := uc.SetEndpointActive(context.Background(), uuid.New(), ep.ID, false)
	if !errors.Is(err, ErrEndpointNotFound) {
		t.Fatalf("cross-tenant toggle: want ErrEndpointNotFound, got %v", err)
	}
	if !store.endpoints[ep.ID].Active {
		t.Fatal("a non-owner toggle must not mutate the endpoint")
	}
	if len(store.activeUpdates) != 0 {
		t.Fatalf("a non-owner toggle must not persist anything, got %+v", store.activeUpdates)
	}
}

func TestPublish_FansOutToSubscribedActiveEndpoints(t *testing.T) {
	store := newFakeStore()
	userID := uuid.New()
	subscribed := mustEndpoint(t, userID, domain.EventLeadCreated, domain.EventLeadQualified)
	other := mustEndpoint(t, userID, domain.EventSequenceCompleted) // not subscribed to lead.created
	inactive := mustEndpoint(t, userID, domain.EventLeadCreated)
	inactive.Active = false
	store.endpoints[subscribed.ID] = subscribed
	store.endpoints[other.ID] = other
	store.endpoints[inactive.ID] = inactive

	uc := NewUseCase(store, &fakeClient{}, cfg(), nil)
	n, err := uc.Publish(context.Background(), userID, domain.EventLeadCreated, []byte(`{"id":"x"}`))
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 delivery enqueued (only the subscribed active endpoint), got %d", n)
	}
	if len(store.deliveries) != 1 || store.deliveries[0].EndpointID != subscribed.ID {
		t.Fatal("delivery must target the subscribed active endpoint")
	}
}

func TestTestEndpoint_EnqueuesPing(t *testing.T) {
	store := newFakeStore()
	owner := uuid.New()
	ep := mustEndpoint(t, owner, domain.EventLeadCreated)
	store.endpoints[ep.ID] = ep
	uc := NewUseCase(store, &fakeClient{}, cfg(), nil)

	if err := uc.TestEndpoint(context.Background(), owner, ep.ID); err != nil {
		t.Fatalf("TestEndpoint: %v", err)
	}
	if len(store.deliveries) != 1 {
		t.Fatalf("expected a ping delivery enqueued, got %d", len(store.deliveries))
	}
	// Cross-tenant test is rejected.
	if err := uc.TestEndpoint(context.Background(), uuid.New(), ep.ID); !errors.Is(err, ErrEndpointNotFound) {
		t.Fatalf("cross-tenant test: want ErrEndpointNotFound, got %v", err)
	}
}

func TestProcessPending_DeliversAndSigns(t *testing.T) {
	store := newFakeStore()
	userID := uuid.New()
	ep := mustEndpoint(t, userID, domain.EventLeadCreated)
	store.endpoints[ep.ID] = ep
	d, _ := domain.NewDelivery(userID, ep.ID, domain.EventLeadCreated, []byte(`{"id":"abc"}`))
	store.deliveries = append(store.deliveries, d)

	client := &fakeClient{status: 200}
	obs := &fakeObserver{}
	uc := NewUseCase(store, client, cfg(), obs)

	n, err := uc.ProcessPending(context.Background())
	if err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 delivered, got %d", n)
	}
	if client.calls != 1 || client.gotURL != "https://example.com/hook" {
		t.Fatal("client must be called with the endpoint URL")
	}
	if client.gotSig != domain.SignPayload(d.Payload, ep.Secret) {
		t.Fatal("payload must be signed with the endpoint secret")
	}
	if client.gotEventID != d.EventID.String() {
		t.Fatal("delivery must carry its EventID as the idempotency key")
	}
	if d.Status != domain.DeliverySucceeded {
		t.Fatalf("delivery status = %q, want succeeded", d.Status)
	}
	if len(obs.events) != 1 || obs.events[0] != "lead.created" || !obs.success[0] {
		t.Fatal("observer must record a successful delivery for the event type")
	}
}

func TestProcessPending_RetryableFailureStaysPending(t *testing.T) {
	store := newFakeStore()
	userID := uuid.New()
	ep := mustEndpoint(t, userID, domain.EventLeadCreated)
	store.endpoints[ep.ID] = ep
	d, _ := domain.NewDelivery(userID, ep.ID, domain.EventLeadCreated, []byte(`{}`))
	store.deliveries = append(store.deliveries, d)

	client := &fakeClient{status: 503, err: errors.New("server error")}
	obs := &fakeObserver{}
	uc := NewUseCase(store, client, cfg(), obs)

	if _, err := uc.ProcessPending(context.Background()); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
	if d.Status != domain.DeliveryPending {
		t.Fatalf("retryable failure: status = %q, want pending", d.Status)
	}
	if d.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", d.Attempts)
	}
	if obs.success[0] {
		t.Fatal("observer must record a failed delivery")
	}
}

func TestProcessPending_EndpointGoneFailsDelivery(t *testing.T) {
	store := newFakeStore()
	userID := uuid.New()
	// Delivery references an endpoint that no longer exists.
	d, _ := domain.NewDelivery(userID, uuid.New(), domain.EventLeadCreated, []byte(`{}`))
	store.deliveries = append(store.deliveries, d)
	client := &fakeClient{status: 200}
	uc := NewUseCase(store, client, cfg(), nil)

	if _, err := uc.ProcessPending(context.Background()); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
	if client.calls != 0 {
		t.Fatal("must not attempt HTTP for a deleted endpoint")
	}
	if d.Status != domain.DeliveryFailed {
		t.Fatalf("status = %q, want failed (endpoint gone, terminal)", d.Status)
	}
}

func TestProcessPending_InactiveEndpointDropsDelivery(t *testing.T) {
	store := newFakeStore()
	userID := uuid.New()
	ep := mustEndpoint(t, userID, domain.EventLeadCreated)
	ep.SetActive(false) // disabled after the delivery was enqueued
	store.endpoints[ep.ID] = ep
	d, _ := domain.NewDelivery(userID, ep.ID, domain.EventLeadCreated, []byte(`{}`))
	store.deliveries = append(store.deliveries, d)

	client := &fakeClient{status: 200}
	uc := NewUseCase(store, client, cfg(), nil)

	delivered, err := uc.ProcessPending(context.Background())
	if err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
	if delivered != 0 {
		t.Fatalf("delivered = %d, want 0 for an inactive endpoint", delivered)
	}
	if client.calls != 0 {
		t.Fatal("must not POST to a disabled endpoint")
	}
	if d.Status != domain.DeliveryFailed {
		t.Fatalf("status = %q, want failed (endpoint inactive, terminal — not left pending to busy-loop)", d.Status)
	}
}
