package ratelimit

import (
	"context"
	"sync"
	"time"
)

// InMemoryLimiter implements Limiter via a sliding-window log kept in
// process memory. It is the default unit-test backend and serves as
// fallback for single-instance deploys when Redis is not configured.
// Per-key counters live until Allow is invoked again for that key — a
// rarely-touched key may keep its stale slice until next call; the
// memory cost is bounded because each slice never exceeds limit+1
// entries by construction.
type InMemoryLimiter struct {
	limit    int
	window   time.Duration
	now      func() time.Time
	mu       sync.Mutex
	requests map[string][]time.Time
}

// NewInMemoryLimiter creates a limiter that allows up to limit calls
// per key within the trailing window. Defaults the clock to time.Now.
func NewInMemoryLimiter(limit int, window time.Duration) *InMemoryLimiter {
	return NewInMemoryLimiterWithClock(limit, window, time.Now)
}

// NewInMemoryLimiterWithClock is the clock-injectable constructor used
// by tests that need to advance virtual time without sleeping.
func NewInMemoryLimiterWithClock(limit int, window time.Duration, now func() time.Time) *InMemoryLimiter {
	return &InMemoryLimiter{
		limit:    limit,
		window:   window,
		now:      now,
		requests: make(map[string][]time.Time),
	}
}

// Allow implements Limiter. Sliding-window log: drop timestamps older
// than now-window, count what remains; permit if below limit, else
// surface the time until the oldest still-counted entry expires.
func (l *InMemoryLimiter) Allow(_ context.Context, key string) (bool, time.Duration, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	cutoff := now.Add(-l.window)
	list := l.requests[key]
	keep := list[:0]
	for _, t := range list {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}
	if len(keep) >= l.limit {
		// Oldest entry leaves the window at keep[0]+window; that is
		// the earliest moment a new request can succeed.
		retryAfter := keep[0].Add(l.window).Sub(now)
		l.requests[key] = keep
		return false, retryAfter, nil
	}
	keep = append(keep, now)
	l.requests[key] = keep
	return true, 0, nil
}
