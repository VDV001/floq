package ratelimit_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/daniil/floq/internal/ratelimit"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func fixedKey(k string) ratelimit.KeyFunc {
	return func(_ *http.Request) (string, bool) { return k, true }
}

func unkeyed() ratelimit.KeyFunc {
	return func(_ *http.Request) (string, bool) { return "", false }
}

type erroringLimiter struct{}

func (erroringLimiter) Allow(_ context.Context, _ string) (bool, time.Duration, error) {
	return false, 0, errors.New("redis down")
}

func TestMiddleware_UnderLimitPassesThrough(t *testing.T) {
	limiter := ratelimit.NewInMemoryLimiter(3, time.Minute)
	mw := ratelimit.Middleware(limiter, fixedKey("u1"), nil)
	handler := mw(okHandler())

	for i := 0; i < 3; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/x", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i+1, rr.Code)
		}
	}
}

func TestMiddleware_AtLimitReturns429WithRetryAfter(t *testing.T) {
	limiter := ratelimit.NewInMemoryLimiter(3, time.Minute)
	mw := ratelimit.Middleware(limiter, fixedKey("u1"), nil)
	handler := mw(okHandler())

	for i := 0; i < 3; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/x", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("warm-up request %d: got %d, want 200", i+1, rr.Code)
		}
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/x", nil))
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("4th request must be 429, got %d", rr.Code)
	}
	raw := rr.Header().Get("Retry-After")
	if raw == "" {
		t.Fatal("429 response must include Retry-After header")
	}
	retryAfter, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("Retry-After %q must be integer seconds: %v", raw, err)
	}
	if retryAfter < 1 || retryAfter > 60 {
		t.Errorf("Retry-After = %d, want within 1..60 (whole seconds, within window)", retryAfter)
	}
}

func TestMiddleware_PerKeyIsolated(t *testing.T) {
	limiter := ratelimit.NewInMemoryLimiter(2, time.Minute)
	handlerA := ratelimit.Middleware(limiter, fixedKey("uA"), nil)(okHandler())
	handlerB := ratelimit.Middleware(limiter, fixedKey("uB"), nil)(okHandler())

	// User A burns through the limit.
	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		handlerA.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/x", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("A request %d: got %d", i+1, rr.Code)
		}
	}
	rr := httptest.NewRecorder()
	handlerA.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/x", nil))
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("A's 3rd request must be blocked, got %d", rr.Code)
	}

	// User B must be untouched.
	rr = httptest.NewRecorder()
	handlerB.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/x", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("user B got %d while user A was at limit — per-key isolation broken", rr.Code)
	}
}

func TestMiddleware_LimiterErrorFailsOpen(t *testing.T) {
	mw := ratelimit.Middleware(erroringLimiter{}, fixedKey("u1"), nil)
	handler := mw(okHandler())

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/x", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("limiter error must fail-open (Redis outage cannot block legitimate operators); got %d", rr.Code)
	}
}

func TestMiddleware_NoKeyBypassesLimit(t *testing.T) {
	// Limit zero means "deny everything that the limiter sees". With
	// an unkeyed request the middleware must NOT consult the limiter
	// at all — defer to the next handler.
	limiter := ratelimit.NewInMemoryLimiter(0, time.Minute)
	mw := ratelimit.Middleware(limiter, unkeyed(), nil)
	handler := mw(okHandler())

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/x", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("no-key request must bypass the limiter (deferred to next layer), got %d", rr.Code)
	}
}

func TestInMemoryLimiter_AfterWindowAllowsAgain(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	limiter := ratelimit.NewInMemoryLimiterWithClock(2, time.Minute, clock)
	ctx := context.Background()

	allowed, _, err := limiter.Allow(ctx, "k")
	if err != nil || !allowed {
		t.Fatalf("1st must allow: allowed=%v err=%v", allowed, err)
	}
	allowed, _, err = limiter.Allow(ctx, "k")
	if err != nil || !allowed {
		t.Fatalf("2nd must allow: allowed=%v err=%v", allowed, err)
	}
	allowed, retryAfter, err := limiter.Allow(ctx, "k")
	if err != nil {
		t.Fatalf("3rd Allow err = %v", err)
	}
	if allowed {
		t.Fatal("3rd must block — limit is 2")
	}
	if retryAfter <= 0 || retryAfter > time.Minute {
		t.Errorf("retryAfter = %v, want 0..1m", retryAfter)
	}

	// Slide the clock past the window — slot frees up.
	now = now.Add(time.Minute + time.Second)
	allowed, _, err = limiter.Allow(ctx, "k")
	if err != nil || !allowed {
		t.Errorf("post-window request must allow: allowed=%v err=%v", allowed, err)
	}
}
