package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/daniil/floq/internal/ratelimit"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// limitMW builds a per-IP rate-limit middleware backed by an in-memory
// sliding window — the same wiring shape the composition root uses, so
// these tests exercise the real KeyFunc + Middleware path.
func limitMW(prefix string, limit int) func(http.Handler) http.Handler {
	l := ratelimit.NewInMemoryLimiter(limit, time.Minute)
	return ratelimit.Middleware(l, ratelimit.IPKeyFunc(prefix), nil)
}

func postJSON(t *testing.T, r chi.Router, path string, v any) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(v)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestRegisterRoutes_LoginRateLimited(t *testing.T) {
	repo := newMockUserRepo()
	h := newTestHandlerWithRepo(repo)
	r := chi.NewRouter()
	RegisterRoutes(r, h, limitMW("ratelimit:test-login:", 2), nil)

	// Seed a real user so wrong-password attempts hit the bcrypt path.
	rec := postJSON(t, r, "/api/auth/register", registerRequest{
		Email: "victim@example.com", Password: "correct-horse", FullName: "Victim",
	})
	require.Equal(t, http.StatusCreated, rec.Code)

	bad := loginRequest{Email: "victim@example.com", Password: "wrong"}

	// First two brute-force attempts are answered normally (401).
	for i := 1; i <= 2; i++ {
		rec := postJSON(t, r, "/api/auth/login", bad)
		require.Equalf(t, http.StatusUnauthorized, rec.Code, "attempt %d", i)
	}

	// Third attempt from the same IP must be rate-limited.
	rec = postJSON(t, r, "/api/auth/login", bad)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "3rd attempt must be 429")

	// Security: a rate-limited response must NOT leak credential state —
	// the attacker should see the generic limit message, not "invalid
	// credentials" (which distinguishes wrong-password from rate-limit).
	assert.NotContains(t, rec.Body.String(), "invalid credentials")
	assert.Contains(t, rec.Body.String(), "rate limit exceeded")
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}

func TestRegisterRoutes_RegisterRateLimited(t *testing.T) {
	repo := newMockUserRepo()
	h := newTestHandlerWithRepo(repo)
	r := chi.NewRouter()
	RegisterRoutes(r, h, nil, limitMW("ratelimit:test-register:", 2))

	// Two distinct sign-ups succeed.
	for i, email := range []string{"a@example.com", "b@example.com"} {
		rec := postJSON(t, r, "/api/auth/register", registerRequest{
			Email: email, Password: "secure123", FullName: "User",
		})
		require.Equalf(t, http.StatusCreated, rec.Code, "signup %d", i+1)
	}

	// Third signup from the same IP is throttled (anti-spam).
	rec := postJSON(t, r, "/api/auth/register", registerRequest{
		Email: "c@example.com", Password: "secure123", FullName: "User",
	})
	assert.Equal(t, http.StatusTooManyRequests, rec.Code, "3rd signup must be 429")
}

func TestRegisterRoutes_LimitersDoNotCrossContaminate(t *testing.T) {
	repo := newMockUserRepo()
	h := newTestHandlerWithRepo(repo)
	r := chi.NewRouter()
	RegisterRoutes(r, h, limitMW("ratelimit:test-login:", 1), limitMW("ratelimit:test-register:", 5))

	// Exhaust the login limiter.
	bad := loginRequest{Email: "nobody@example.com", Password: "x"}
	require.Equal(t, http.StatusUnauthorized, postJSON(t, r, "/api/auth/login", bad).Code)
	require.Equal(t, http.StatusTooManyRequests, postJSON(t, r, "/api/auth/login", bad).Code)

	// Register must be unaffected by the exhausted login bucket.
	rec := postJSON(t, r, "/api/auth/register", registerRequest{
		Email: "fresh@example.com", Password: "secure123", FullName: "Fresh",
	})
	assert.Equal(t, http.StatusCreated, rec.Code, "register bucket must be independent of login bucket")
}
