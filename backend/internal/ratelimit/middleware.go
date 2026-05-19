package ratelimit

import (
	"log/slog"
	"net/http"
)

// KeyFunc resolves a request to its rate-limit bucket key.
type KeyFunc func(r *http.Request) (key string, ok bool)

// Middleware stub: passes everything through. Replaced by the GREEN
// feat commit with allow/deny + 429 + Retry-After logic.
func Middleware(_ Limiter, _ KeyFunc, _ *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return next
	}
}
