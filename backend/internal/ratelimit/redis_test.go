//go:build integration

package ratelimit_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/daniil/floq/internal/ratelimit"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redisDSN mirrors the rest of the integration suite — local docker-
// compose redis on 6379. REDIS_URL env override is honoured so CI can
// point at its own service container.
func redisDSN() string {
	if v := os.Getenv("REDIS_URL"); v != "" {
		return v
	}
	return "redis://localhost:6379"
}

func testRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	opt, err := redis.ParseURL(redisDSN())
	require.NoError(t, err, "parse redis DSN")
	client := redis.NewClient(opt)
	require.NoError(t, client.Ping(context.Background()).Err(), "ping redis")
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// uniqueKey isolates test buckets so parallel test runs (and reruns
// without manual FLUSHDB) cannot collide. Cleanup deletes the bucket
// after the test even on failure.
func uniqueKey(t *testing.T, client *redis.Client, suffix string) string {
	t.Helper()
	key := "ratelimit-test:" + t.Name() + ":" + suffix
	t.Cleanup(func() { _ = client.Del(context.Background(), key).Err() })
	return key
}

func TestRedisLimiter_AllowsUnderLimit(t *testing.T) {
	client := testRedisClient(t)
	limiter := ratelimit.NewRedisLimiter(client, 3, time.Minute)
	key := uniqueKey(t, client, "under")
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		allowed, retryAfter, err := limiter.Allow(ctx, key)
		require.NoError(t, err)
		assert.True(t, allowed, "request %d must be allowed (under limit)", i+1)
		assert.Equal(t, time.Duration(0), retryAfter, "allowed request must report zero retry-after")
	}
}

func TestRedisLimiter_BlocksAtLimit(t *testing.T) {
	client := testRedisClient(t)
	limiter := ratelimit.NewRedisLimiter(client, 3, time.Minute)
	key := uniqueKey(t, client, "block")
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		allowed, _, err := limiter.Allow(ctx, key)
		require.NoError(t, err)
		require.True(t, allowed, "warm-up %d must be allowed", i+1)
	}

	allowed, retryAfter, err := limiter.Allow(ctx, key)
	require.NoError(t, err)
	assert.False(t, allowed, "4th request must be blocked")
	assert.Greater(t, retryAfter, time.Duration(0), "blocked request must report positive retry-after")
	assert.LessOrEqual(t, retryAfter, time.Minute, "retry-after cannot exceed the window")
}

func TestRedisLimiter_PerKeyIsolated(t *testing.T) {
	client := testRedisClient(t)
	limiter := ratelimit.NewRedisLimiter(client, 2, time.Minute)
	keyA := uniqueKey(t, client, "userA")
	keyB := uniqueKey(t, client, "userB")
	ctx := context.Background()

	// Burn through A's limit.
	for i := 0; i < 2; i++ {
		allowed, _, err := limiter.Allow(ctx, keyA)
		require.NoError(t, err)
		require.True(t, allowed)
	}
	allowed, _, err := limiter.Allow(ctx, keyA)
	require.NoError(t, err)
	require.False(t, allowed, "A's 3rd must be blocked")

	// B must still pass.
	allowed, _, err = limiter.Allow(ctx, keyB)
	require.NoError(t, err)
	assert.True(t, allowed, "B must not be affected by A's exhausted bucket")
}

func TestRedisLimiter_ShortWindowExpires(t *testing.T) {
	// Real-clock test of the sliding-window expiry path. Uses a small
	// window (200 ms) so the test finishes in under a second.
	client := testRedisClient(t)
	limiter := ratelimit.NewRedisLimiter(client, 1, 200*time.Millisecond)
	key := uniqueKey(t, client, "expire")
	ctx := context.Background()

	allowed, _, err := limiter.Allow(ctx, key)
	require.NoError(t, err)
	require.True(t, allowed, "1st must allow")

	allowed, _, err = limiter.Allow(ctx, key)
	require.NoError(t, err)
	require.False(t, allowed, "2nd must block — limit is 1")

	time.Sleep(250 * time.Millisecond)

	allowed, _, err = limiter.Allow(ctx, key)
	require.NoError(t, err)
	assert.True(t, allowed, "post-window request must be allowed again")
}

func TestRedisLimiter_SameMillisecondRequestsDoNotCollideOnZADDMember(t *testing.T) {
	// Subtle Redis ZSET bug if member == score: two requests in the
	// same millisecond would overwrite each other and the second
	// would escape the count. Use a nonce-per-request member to keep
	// every Allow distinct. Replay by firing 5 requests in a tight
	// loop with limit=3 — exactly 3 must be allowed, 2 blocked.
	client := testRedisClient(t)
	limiter := ratelimit.NewRedisLimiter(client, 3, time.Minute)
	key := uniqueKey(t, client, "tight")
	ctx := context.Background()

	allowedCount := 0
	for i := 0; i < 5; i++ {
		allowed, _, err := limiter.Allow(ctx, key)
		require.NoError(t, err)
		if allowed {
			allowedCount++
		}
	}
	assert.Equal(t, 3, allowedCount, "exactly limit-count requests must be allowed; collisions would let extras escape")
}
