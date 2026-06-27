package webhooks

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/webhooks/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func setupWebhookRouter(uc *UseCase, userID uuid.UUID) chi.Router {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := httputil.WithUserID(req.Context(), userID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	RegisterRoutes(r, uc)
	return r
}

func setupWebhookRouterNoAuth(uc *UseCase) chi.Router {
	r := chi.NewRouter()
	RegisterRoutes(r, uc)
	return r
}

func doReq(router chi.Router, method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func TestHandler_CreateEndpoint(t *testing.T) {
	store := newFakeStore()
	uc := NewUseCase(store, &fakeClient{}, cfg(), nil)
	userID := uuid.New()
	router := setupWebhookRouter(uc, userID)

	rr := doReq(router, "POST", "/api/webhooks", map[string]any{
		"url":     "https://example.com/hook",
		"events":  []string{"lead.created", "lead.qualified"},
		"secret":  "supersecretvalue123",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}
	if len(store.endpoints) != 1 {
		t.Fatal("endpoint not persisted")
	}
	// The secret must never be echoed back in the response.
	if strings.Contains(rr.Body.String(), "supersecretvalue123") {
		t.Fatal("response leaked the signing secret")
	}
}

func TestHandler_CreateEndpoint_BadInput(t *testing.T) {
	uc := NewUseCase(newFakeStore(), &fakeClient{}, cfg(), nil)
	router := setupWebhookRouter(uc, uuid.New())

	// SSRF / invalid URL → 400.
	rr := doReq(router, "POST", "/api/webhooks", map[string]any{
		"url": "http://127.0.0.1/x", "events": []string{"lead.created"}, "secret": "supersecretvalue123",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad url: status = %d, want 400", rr.Code)
	}
	// Unknown event → 400.
	rr = doReq(router, "POST", "/api/webhooks", map[string]any{
		"url": "https://x.com/h", "events": []string{"lead.boom"}, "secret": "supersecretvalue123",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("unknown event: status = %d, want 400", rr.Code)
	}
}

func TestHandler_CreateEndpoint_Unauthenticated(t *testing.T) {
	uc := NewUseCase(newFakeStore(), &fakeClient{}, cfg(), nil)
	router := setupWebhookRouterNoAuth(uc)
	rr := doReq(router, "POST", "/api/webhooks", map[string]any{
		"url": "https://x.com/h", "events": []string{"lead.created"}, "secret": "supersecretvalue123",
	})
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestHandler_ListEndpoints(t *testing.T) {
	store := newFakeStore()
	userID := uuid.New()
	ep := mustEndpoint(t, userID, domain.EventLeadCreated)
	store.endpoints[ep.ID] = ep
	uc := NewUseCase(store, &fakeClient{}, cfg(), nil)
	router := setupWebhookRouter(uc, userID)

	rr := doReq(router, "GET", "/api/webhooks", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), ep.ID.String()) {
		t.Fatal("list did not include the endpoint")
	}
	if strings.Contains(rr.Body.String(), ep.Secret) {
		t.Fatal("list leaked the signing secret")
	}
}

func TestHandler_DeleteEndpoint_Ownership(t *testing.T) {
	store := newFakeStore()
	owner := uuid.New()
	ep := mustEndpoint(t, owner, domain.EventLeadCreated)
	store.endpoints[ep.ID] = ep
	uc := NewUseCase(store, &fakeClient{}, cfg(), nil)

	// Non-owner → 404.
	other := setupWebhookRouter(uc, uuid.New())
	if rr := doReq(other, "DELETE", "/api/webhooks/"+ep.ID.String(), nil); rr.Code != http.StatusNotFound {
		t.Fatalf("non-owner delete: status = %d, want 404", rr.Code)
	}
	// Owner → 204.
	own := setupWebhookRouter(uc, owner)
	if rr := doReq(own, "DELETE", "/api/webhooks/"+ep.ID.String(), nil); rr.Code != http.StatusNoContent {
		t.Fatalf("owner delete: status = %d, want 204", rr.Code)
	}
}

func TestHandler_SetActive(t *testing.T) {
	store := newFakeStore()
	owner := uuid.New()
	ep := mustEndpoint(t, owner, domain.EventLeadCreated)
	store.endpoints[ep.ID] = ep
	uc := NewUseCase(store, &fakeClient{}, cfg(), nil)

	// Owner deactivates → 200, response reflects the new state, store updated.
	own := setupWebhookRouter(uc, owner)
	rr := doReq(own, "PATCH", "/api/webhooks/"+ep.ID.String(), map[string]any{"active": false})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if store.endpoints[ep.ID].Active {
		t.Fatal("endpoint should be deactivated")
	}
	if !strings.Contains(rr.Body.String(), `"active":false`) {
		t.Fatalf("response should report active=false; body=%s", rr.Body.String())
	}
	// Non-owner cannot toggle → 404 (anti-enumeration), endpoint unchanged.
	other := setupWebhookRouter(uc, uuid.New())
	if rr := doReq(other, "PATCH", "/api/webhooks/"+ep.ID.String(), map[string]any{"active": true}); rr.Code != http.StatusNotFound {
		t.Fatalf("non-owner toggle: status = %d, want 404", rr.Code)
	}
	if store.endpoints[ep.ID].Active {
		t.Fatal("non-owner toggle must not reactivate the endpoint")
	}
}

func TestHandler_SetActive_MissingActiveField(t *testing.T) {
	store := newFakeStore()
	owner := uuid.New()
	ep := mustEndpoint(t, owner, domain.EventLeadCreated)
	store.endpoints[ep.ID] = ep
	uc := NewUseCase(store, &fakeClient{}, cfg(), nil)

	own := setupWebhookRouter(uc, owner)
	// A PATCH body without "active" must be rejected (no omit/false ambiguity),
	// not silently disable the endpoint.
	rr := doReq(own, "PATCH", "/api/webhooks/"+ep.ID.String(), map[string]any{})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing active: status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	if !store.endpoints[ep.ID].Active {
		t.Fatal("a body without active must not change the endpoint")
	}
}

func TestHandler_SetActive_Unauthenticated(t *testing.T) {
	store := newFakeStore()
	ep := mustEndpoint(t, uuid.New(), domain.EventLeadCreated)
	store.endpoints[ep.ID] = ep
	uc := NewUseCase(store, &fakeClient{}, cfg(), nil)
	router := setupWebhookRouterNoAuth(uc)
	rr := doReq(router, "PATCH", "/api/webhooks/"+ep.ID.String(), map[string]any{"active": false})
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestHandler_EventTypes(t *testing.T) {
	uc := NewUseCase(newFakeStore(), &fakeClient{}, cfg(), nil)
	router := setupWebhookRouter(uc, uuid.New())

	rr := doReq(router, "GET", "/api/webhooks/event-types", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	// Every known event must be advertised so the UI can build its picker.
	for _, et := range domain.KnownEventTypes() {
		if !strings.Contains(rr.Body.String(), string(et)) {
			t.Errorf("event-types response missing %q", et)
		}
	}
}

func TestHandler_TestEndpoint(t *testing.T) {
	store := newFakeStore()
	owner := uuid.New()
	ep := mustEndpoint(t, owner, domain.EventLeadCreated)
	store.endpoints[ep.ID] = ep
	uc := NewUseCase(store, &fakeClient{}, cfg(), nil)

	own := setupWebhookRouter(uc, owner)
	rr := doReq(own, "POST", "/api/webhooks/"+ep.ID.String()+"/test", nil)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rr.Code)
	}
	if len(store.deliveries) != 1 {
		t.Fatal("test ping was not enqueued")
	}
	// Non-owner → 404.
	other := setupWebhookRouter(uc, uuid.New())
	if rr := doReq(other, "POST", "/api/webhooks/"+ep.ID.String()+"/test", nil); rr.Code != http.StatusNotFound {
		t.Fatalf("non-owner test: status = %d, want 404", rr.Code)
	}
}
