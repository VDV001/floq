package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daniil/floq/internal/metrics"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// routerWith mounts the metrics middleware on a chi router so requests
// carry a RouteContext (the middleware reads the matched route pattern,
// not the raw path, to keep label cardinality bounded).
func routerWith(m *metrics.Metrics) chi.Router {
	r := chi.NewRouter()
	r.Use(m.HTTPMiddleware)
	r.Get("/api/leads/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Post("/api/boom", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	return r
}

func TestHTTPMiddleware_CountsRequestByRoutePatternMethodStatus(t *testing.T) {
	m := metrics.New()
	r := routerWith(m)

	// Two hits on the same parameterised route — must collapse onto the
	// pattern label, not two distinct id-bearing series.
	for _, id := range []string{"111", "222"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/leads/"+id, nil))
		require.Equal(t, http.StatusOK, rec.Code)
	}

	got := testutil.ToFloat64(m.RequestsCounter("/api/leads/{id}", http.MethodGet, "200"))
	assert.Equal(t, 2.0, got, "both hits collapse onto the route pattern")
}

func TestHTTPMiddleware_RecordsErrorStatus(t *testing.T) {
	m := metrics.New()
	r := routerWith(m)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/boom", nil))
	require.Equal(t, http.StatusInternalServerError, rec.Code)

	assert.Equal(t, 1.0, testutil.ToFloat64(m.RequestsCounter("/api/boom", http.MethodPost, "500")))
	// The OK route must not have been touched.
	assert.Equal(t, 0.0, testutil.ToFloat64(m.RequestsCounter("/api/leads/{id}", http.MethodGet, "200")))
}

func TestHTTPMiddleware_PreservesResponseWriterCapabilities(t *testing.T) {
	m := metrics.New()
	var sawFlusher bool
	r := chi.NewRouter()
	r.Use(m.HTTPMiddleware)
	r.Get("/stream", func(w http.ResponseWriter, _ *http.Request) {
		// httptest.ResponseRecorder implements http.Flusher; the metrics
		// wrapper must not hide it from downstream handlers (SSE/stream).
		_, sawFlusher = w.(http.Flusher)
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stream", nil))

	assert.True(t, sawFlusher, "wrapped ResponseWriter must still expose http.Flusher")
}

func TestHTTPMiddleware_SkipsMetricsEndpointSelf(t *testing.T) {
	m := metrics.New()
	r := chi.NewRouter()
	r.Use(m.HTTPMiddleware)
	r.Handle("/metrics", m.Handler())

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	// The scrape endpoint must not observe itself — that would inflate
	// counters on every Prometheus pull.
	assert.Equal(t, 0.0, testutil.ToFloat64(m.RequestsCounter("/metrics", http.MethodGet, "200")))
}
