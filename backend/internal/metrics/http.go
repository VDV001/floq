package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// HTTPMiddleware records request count and latency, labelled by the chi
// route PATTERN (e.g. "/api/leads/{id}", never the id-bearing path) so
// label cardinality stays bounded. Mount it with r.Use on the chi
// router so the request carries a RouteContext.
//
// The scrape endpoint (/metrics) is skipped: counting it would inflate
// every series on each Prometheus pull and tell us nothing useful.
func (m *Metrics) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		// RoutePattern is populated by chi as it routes; read it after
		// the handler ran so nested mounts contribute their full path.
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "/metrics" {
			return
		}
		if route == "" {
			// No route matched (404 on an unregistered path). Bucket
			// these under a single label so a scanner probing random
			// URLs cannot explode cardinality.
			route = "unmatched"
		}

		status := strconv.Itoa(rec.status)
		m.httpRequests.WithLabelValues(route, r.Method, status).Inc()
		m.httpDuration.WithLabelValues(route, r.Method, status).Observe(time.Since(start).Seconds())
	})
}

// statusRecorder captures the response status code. It defaults to 200
// because a handler that writes a body without calling WriteHeader
// implicitly sends 200.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.wroteHeader = true // an implicit 200 is now locked in
	return r.ResponseWriter.Write(b)
}
