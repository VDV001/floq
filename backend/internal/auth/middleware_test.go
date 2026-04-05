package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/daniil/floq/internal/httputil"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-jwt-secret-key"

func makeToken(t *testing.T, claims jwt.MapClaims, secret string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return s
}

func makeValidAccessToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	return makeToken(t, jwt.MapClaims{
		"user_id": userID.String(),
		"type":    "access",
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(time.Hour).Unix(),
	}, testSecret)
}

func TestAuthMiddleware_ValidJWT(t *testing.T) {
	userID := uuid.New()
	token := makeValidAccessToken(t, userID)

	var capturedUserID uuid.UUID
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := httputil.UserIDFromContext(r.Context())
		assert.True(t, ok)
		capturedUserID = id
		w.WriteHeader(http.StatusOK)
	})

	mw := AuthMiddleware(testSecret)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, userID, capturedUserID)
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	mw := AuthMiddleware(testSecret)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing authorization header")
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	token := makeToken(t, jwt.MapClaims{
		"user_id": uuid.New().String(),
		"type":    "access",
		"iat":     time.Now().Add(-2 * time.Hour).Unix(),
		"exp":     time.Now().Add(-1 * time.Hour).Unix(),
	}, testSecret)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	mw := AuthMiddleware(testSecret)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid token")
}

func TestAuthMiddleware_MalformedToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	mw := AuthMiddleware(testSecret)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.jwt")
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthMiddleware_RefreshTokenRejected(t *testing.T) {
	// A refresh token should be rejected by the middleware (it only accepts "access" type).
	token := makeToken(t, jwt.MapClaims{
		"user_id": uuid.New().String(),
		"type":    "refresh",
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(time.Hour).Unix(),
	}, testSecret)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	mw := AuthMiddleware(testSecret)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid token type")
}

func TestAuthMiddleware_WrongSecret(t *testing.T) {
	token := makeToken(t, jwt.MapClaims{
		"user_id": uuid.New().String(),
		"type":    "access",
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(time.Hour).Unix(),
	}, "wrong-secret")

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	mw := AuthMiddleware(testSecret)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthMiddleware_InvalidAuthScheme(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	mw := AuthMiddleware(testSecret)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid authorization header")
}

// Helper to parse JSON error responses.
func parseErrorResponse(t *testing.T, body []byte) map[string]string {
	t.Helper()
	var m map[string]string
	err := json.Unmarshal(body, &m)
	require.NoError(t, err)
	return m
}
