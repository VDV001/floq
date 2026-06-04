package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// HTTPMiddleware records request count and latency, labelled by the chi
// route PATTERN (e.g. "/api/leads/{id}", never the id-bearing path) so
// label cardinality stays bounded. Mount it with r.Use on the chi
// router so the request carries a RouteContext.
//
// It wraps the writer with chi's WrapResponseWriter (not a bespoke
// recorder) so the status code is captured WITHOUT hiding the optional
// ResponseWriter capabilities (Flusher/Hijacker/ReaderFrom/Pusher) from
// downstream handlers and middleware — a bespoke embedder would mask
// them and break SSE/streaming/sendfile.
//
// The scrape endpoint (/metrics) is skipped: counting it would inflate
// every series on each Prometheus pull and tell us nothing useful.
func (m *Metrics) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

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

		// A handler that writes a body (or nothing) without calling
		// WriteHeader implicitly sends 200; WrapResponseWriter reports
		// status 0 in that case, so normalise it.
		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}

		statusLabel := strconv.Itoa(status)
		m.httpRequests.WithLabelValues(route, r.Method, statusLabel).Inc()
		m.httpDuration.WithLabelValues(route, r.Method, statusLabel).Observe(time.Since(start).Seconds())
	})
}
