package ratelimit

import (
	"context"
	"errors"
	"time"
)

// InMemoryLimiter implements Limiter (stub awaiting GREEN feat commit).
type InMemoryLimiter struct {
	limit  int
	window time.Duration
}

// NewInMemoryLimiter returns a stub that always errors so tests fail RED
// until the GREEN commit lands a real sliding-window implementation.
func NewInMemoryLimiter(limit int, window time.Duration) *InMemoryLimiter {
	return &InMemoryLimiter{limit: limit, window: window}
}

// NewInMemoryLimiterWithClock — same stub, clock-injectable shape.
func NewInMemoryLimiterWithClock(limit int, window time.Duration, _ func() time.Time) *InMemoryLimiter {
	return &InMemoryLimiter{limit: limit, window: window}
}

// Allow stub: always errors so the middleware path takes its
// error-handling branch.
func (l *InMemoryLimiter) Allow(_ context.Context, _ string) (bool, time.Duration, error) {
	return false, 0, errors.New("ratelimit: NewInMemoryLimiter not implemented")
}
