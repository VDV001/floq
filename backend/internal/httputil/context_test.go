package httputil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestUserIDContext(t *testing.T) {
	t.Run("round-trip store and retrieve", func(t *testing.T) {
		id := uuid.New()
		ctx := WithUserID(context.Background(), id)
		got, ok := UserIDFromContext(ctx)
		if !ok {
			t.Fatal("UserIDFromContext returned ok=false")
		}
		if got != id {
			t.Errorf("got %v, want %v", got, id)
		}
	})

	t.Run("missing returns zero and false", func(t *testing.T) {
		got, ok := UserIDFromContext(context.Background())
		if ok {
			t.Error("expected ok=false for empty context")
		}
		if got != uuid.Nil {
			t.Errorf("got %v, want uuid.Nil", got)
		}
	})
}

func TestParseIDParam(t *testing.T) {
	t.Run("valid uuid", func(t *testing.T) {
		id := uuid.New()
		r := buildRequestWithChiParam("id", id.String())

		got, err := ParseIDParam(r, "id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != id {
			t.Errorf("got %v, want %v", got, id)
		}
	})

	t.Run("invalid uuid", func(t *testing.T) {
		r := buildRequestWithChiParam("id", "not-a-uuid")
		_, err := ParseIDParam(r, "id")
		if err == nil {
			t.Fatal("expected error for invalid uuid")
		}
	})

	t.Run("missing param returns error", func(t *testing.T) {
		r := buildRequestWithChiParam("other", uuid.New().String())
		_, err := ParseIDParam(r, "id")
		if err == nil {
			t.Fatal("expected error for missing param")
		}
	})
}

// buildRequestWithChiParam creates a request with a chi URL parameter set.
func buildRequestWithChiParam(key, value string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
