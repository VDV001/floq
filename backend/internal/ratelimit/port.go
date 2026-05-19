// Package ratelimit provides a small per-key rate-limiter port and HTTP
// middleware used to cap abuse on sensitive endpoints (initially the
// HITL approve/reject pair). The port is intentionally narrow so an
// in-memory adapter can satisfy unit tests and a Redis-backed adapter
// can be plugged in at composition root for multi-instance deploys.
package ratelimit

import (
	"context"
	"time"
)

// Limiter is the abstract token-account check. Allow returns whether
// the request keyed by key is permitted right now and, when denied,
// the duration the caller should wait before retrying. Errors are
// considered transient: the middleware fails open so an outage of the
// backing store cannot lock legitimate operators out.
type Limiter interface {
	Allow(ctx context.Context, key string) (allowed bool, retryAfter time.Duration, err error)
}
