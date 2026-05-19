package ratelimit

import (
	"context"
	"errors"
	"time"
)

// RedisLimiter — STUB awaiting GREEN feat commit. Wire shape is fixed
// so the integration test file can reference it; Allow returns an
// error so middleware fail-open kicks in until the real Lua impl
// lands.
type RedisLimiter struct {
	limit  int
	window time.Duration
}

// NewRedisLimiter stub: ignores the client so the package compiles
// without a redis import on the RED step.
func NewRedisLimiter(_ any, limit int, window time.Duration) *RedisLimiter {
	return &RedisLimiter{limit: limit, window: window}
}

// Allow stub: always errors.
func (l *RedisLimiter) Allow(_ context.Context, _ string) (bool, time.Duration, error) {
	return false, 0, errors.New("ratelimit: RedisLimiter not implemented")
}
