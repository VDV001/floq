package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// slidingWindowLua atomically drops entries older than the trailing
// window, checks the remaining count against the limit, and either
// records the new request (member = unique nonce so two requests in
// the same millisecond cannot collide) or returns the milliseconds
// the caller should wait before retrying.
//
// KEYS[1]  bucket key
// ARGV[1]  now (unix ms)
// ARGV[2]  window (ms)
// ARGV[3]  limit
// ARGV[4]  per-request nonce (any unique string)
// returns  0 when allowed, >0 = retry-after ms when denied
const slidingWindowLua = `
local key    = KEYS[1]
local now    = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit  = tonumber(ARGV[3])
local nonce  = ARGV[4]

redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)
local count = redis.call('ZCARD', key)
if count >= limit then
  local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
  local oldest_score = tonumber(oldest[2])
  local retry = oldest_score + window - now
  if retry < 1 then retry = 1 end
  return retry
end
redis.call('ZADD', key, now, nonce)
redis.call('PEXPIRE', key, window)
return 0
`

// RedisLimiter implements Limiter via a Redis ZSET acting as a
// sliding-window log. The full check is one round-trip atomic Lua
// invocation so concurrent Allow calls cannot race past the limit.
type RedisLimiter struct {
	client redis.UniversalClient
	limit  int
	window time.Duration
	script *redis.Script
	now    func() time.Time
}

// NewRedisLimiter wires the limiter to a redis.UniversalClient (the
// interface satisfied by both *redis.Client and *redis.ClusterClient).
// Accepting `any` keeps the package free of forced type-assertions at
// the call site — main.go passes its *redis.Client and the script
// dispatcher does the rest.
func NewRedisLimiter(client any, limit int, window time.Duration) *RedisLimiter {
	uc, ok := client.(redis.UniversalClient)
	if !ok {
		// At construction time we cannot return an error — but the
		// stub branch keeps the package safe: Allow will return an
		// error, middleware fails open, and ops dashboards see the
		// log line. main.go MUST pass a real client.
		return &RedisLimiter{limit: limit, window: window}
	}
	return &RedisLimiter{
		client: uc,
		limit:  limit,
		window: window,
		script: redis.NewScript(slidingWindowLua),
		now:    time.Now,
	}
}

// Allow consults the Lua script. Errors from the client propagate so
// the middleware can fail open and a Redis outage does not cascade
// into operator lockout.
func (l *RedisLimiter) Allow(ctx context.Context, key string) (bool, time.Duration, error) {
	if l.client == nil || l.script == nil {
		return false, 0, fmt.Errorf("ratelimit: RedisLimiter constructed without a redis client")
	}
	nowMs := l.now().UnixMilli()
	windowMs := l.window.Milliseconds()
	nonce := uuid.NewString()
	res, err := l.script.Run(ctx, l.client, []string{key}, nowMs, windowMs, l.limit, nonce).Int64()
	if err != nil {
		return false, 0, fmt.Errorf("ratelimit: redis script: %w", err)
	}
	if res == 0 {
		return true, 0, nil
	}
	return false, time.Duration(res) * time.Millisecond, nil
}
