package onec_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/integrations/onec"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// fakeResolver is a SecretResolver for middleware tests.
type fakeResolver struct {
	user  uuid.UUID
	found bool
	err   error
}

func (f *fakeResolver) UserIDByWebhookSecret(_ context.Context, _ string) (uuid.UUID, bool, error) {
	return f.user, f.found, f.err
}

const validBody = `{"external_id":"ОП-1","external_type":"Документ.Оплата","kind":"payment","payload":{"sum":1000}}`

func newHandler(store onec.SyncStore) *onec.Handler {
	return onec.NewHandler(onec.NewUseCase(store))
}

// --- handler (assumes auth middleware already put userID in context) ---

func TestWebhook_Valid(t *testing.T) {
	store := &fakeStore{inserted: true}
	h := newHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/api/integrations/onec/webhook", strings.NewReader(validBody))
	req = req.WithContext(httputil.WithUserID(req.Context(), uuid.New()))
	rec := httptest.NewRecorder()
	h.Webhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if store.calls != 1 {
		t.Errorf("store calls = %d, want 1", store.calls)
	}
}

func TestWebhook_DuplicateReturns200(t *testing.T) {
	store := &fakeStore{inserted: false} // dedup hit
	h := newHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(validBody))
	req = req.WithContext(httputil.WithUserID(req.Context(), uuid.New()))
	rec := httptest.NewRecorder()
	h.Webhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("replay status = %d, want 200", rec.Code)
	}
}

func TestWebhook_InvalidKind(t *testing.T) {
	store := &fakeStore{inserted: true}
	h := newHandler(store)

	body := `{"external_id":"ОП-1","external_type":"Документ","kind":"delivery","payload":{}}`
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
	req = req.WithContext(httputil.WithUserID(req.Context(), uuid.New()))
	rec := httptest.NewRecorder()
	h.Webhook(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if store.calls != 0 {
		t.Errorf("store must not be called on invalid kind")
	}
}

func TestWebhook_BadJSON(t *testing.T) {
	h := newHandler(&fakeStore{})
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{not json`))
	req = req.WithContext(httputil.WithUserID(req.Context(), uuid.New()))
	rec := httptest.NewRecorder()
	h.Webhook(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestWebhook_NoUserInContext(t *testing.T) {
	h := newHandler(&fakeStore{})
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(validBody))
	rec := httptest.NewRecorder()
	h.Webhook(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// --- secret middleware (tenant resolution by webhook secret) ---

func TestWebhookAuth_MissingSecret(t *testing.T) {
	mw := onec.WebhookAuth(&fakeResolver{found: false})
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestWebhookAuth_UnknownSecret(t *testing.T) {
	mw := onec.WebhookAuth(&fakeResolver{found: false})
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("X-Onec-Secret", "nope")
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestWebhookAuth_ValidSecretInjectsUser(t *testing.T) {
	want := uuid.New()
	mw := onec.WebhookAuth(&fakeResolver{user: want, found: true})

	var got uuid.UUID
	var ok bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = httputil.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("X-Onec-Secret", "good")
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !ok || got != want {
		t.Fatalf("userID in ctx = %v (ok=%v), want %v", got, ok, want)
	}
}

func TestWebhookAuth_ResolverError(t *testing.T) {
	mw := onec.WebhookAuth(&fakeResolver{err: errors.New("db down")})
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("X-Onec-Secret", "good")
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// --- full wire-up: RegisterRoutes composes auth middleware + handler ---

func TestRegisterRoutes_EndToEnd(t *testing.T) {
	store := &fakeStore{inserted: true}
	resolver := &fakeResolver{user: uuid.New(), found: true}
	r := chi.NewRouter()
	onec.RegisterRoutes(r, onec.NewHandler(onec.NewUseCase(store)), resolver)

	const path = "/api/integrations/onec/webhook"

	t.Run("valid secret + body → 200, stored", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(validBody))
		req.Header.Set("X-Onec-Secret", "good")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
		}
		if store.calls != 1 {
			t.Errorf("store calls = %d, want 1", store.calls)
		}
	})

	t.Run("missing secret → 401, not stored", func(t *testing.T) {
		store.calls = 0
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(validBody))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		if store.calls != 0 {
			t.Errorf("unauthenticated request must not reach the store")
		}
	})
}

// An event with no kind whose external type isn't mapped can't be classified →
// 422 (so 1C surfaces it as unprocessable rather than retrying forever).
func TestWebhook_UnmappedNoKindReturns422(t *testing.T) {
	store := &fakeStore{inserted: true}
	mapping := &fakeMapping{err: onec.ErrMappingNotFound} // no active mapping
	h := onec.NewHandler(onec.NewUseCase(store, onec.WithMapping(mapping)))

	body := `{"external_id":"X-1","external_type":"Документ.Неизвестный","payload":{}}`
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
	req = req.WithContext(httputil.WithUserID(req.Context(), uuid.New()))
	rec := httptest.NewRecorder()
	h.Webhook(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body=%s", rec.Code, rec.Body.String())
	}
	if store.calls != 0 {
		t.Error("unclassifiable event must not be recorded")
	}
}

// Oversized JSON body under JSONBodyCap must yield 413, not 400 — the public
// webhook is the only unauthenticated-by-JWT route and must reject large
// bodies with the right status.
func TestWebhook_BodyCapReturns413(t *testing.T) {
	store := &fakeStore{inserted: true}
	r := chi.NewRouter()
	r.Use(httputil.JSONBodyCap(64)) // tiny cap to force overflow
	resolver := &fakeResolver{user: uuid.New(), found: true}
	onec.RegisterRoutes(r, onec.NewHandler(onec.NewUseCase(store)), resolver)

	big := `{"external_id":"x","external_type":"y","kind":"payment","payload":"` + strings.Repeat("A", 500) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/onec/webhook", strings.NewReader(big))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Onec-Secret", "good")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}
