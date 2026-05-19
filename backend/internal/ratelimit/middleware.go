package ratelimit

import (
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/daniil/floq/internal/httputil"
)

// KeyFunc resolves a request to its rate-limit bucket key. Returning
// ok=false means "do not rate-limit this request" — the middleware
// defers to the next handler. Production callers typically build the
// key from the authenticated user_id so anonymous traffic (which
// should not reach a rate-limited route in the first place) bypasses
// the bucket rather than hammering one shared "anonymous" bucket.
type KeyFunc func(r *http.Request) (key string, ok bool)

// Middleware wires Limiter into the chi handler chain. On allow it
// calls the next handler; on deny it answers 429 with a Retry-After
// header (whole seconds, rounded up so a tight client retry loop
// cannot beat the window edge). On Limiter error it logs and FAILS
// OPEN: a Redis outage must not lock operators out of approving
// urgent drafts. logger may be nil — slog.Default is used as fallback.
func Middleware(l Limiter, keyFn KeyFunc, logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key, ok := keyFn(r)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			allowed, retryAfter, err := l.Allow(r.Context(), key)
			if err != nil {
				logger.ErrorContext(r.Context(), "rate-limit check failed; failing open",
					slog.String("key", key),
					slog.Any("err", err))
				next.ServeHTTP(w, r)
				return
			}
			if !allowed {
				retrySec := int(math.Ceil(retryAfter.Seconds()))
				if retrySec < 1 {
					retrySec = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(retrySec))
				httputil.WriteError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
