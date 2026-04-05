package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests focus on input validation (no DB required) and JWT token operations.

func newTestHandler() *Handler {
	return NewHandler(nil, testSecret)
}

// --- Register tests (input validation, no DB needed) ---

func TestRegister_MissingFields(t *testing.T) {
	h := newTestHandler()

	tests := []struct {
		name string
		body registerRequest
	}{
		{"missing email", registerRequest{Email: "", Password: "123456", FullName: "Test"}},
		{"missing password", registerRequest{Email: "a@b.com", Password: "", FullName: "Test"}},
		{"missing full_name", registerRequest{Email: "a@b.com", Password: "123456", FullName: ""}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
			rec := httptest.NewRecorder()

			h.Register(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), "email, password and full_name are required")
		})
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	h := newTestHandler()

	body, _ := json.Marshal(registerRequest{
		Email:    "test@example.com",
		Password: "12345",
		FullName: "Test User",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "password must be at least 6 characters")
}

func TestRegister_InvalidJSON(t *testing.T) {
	h := newTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid request body")
}

// --- Login tests (input validation, no DB needed) ---

func TestLogin_MissingEmail(t *testing.T) {
	h := newTestHandler()

	body, _ := json.Marshal(loginRequest{Email: "", Password: "123456"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "email and password are required")
}

func TestLogin_MissingPassword(t *testing.T) {
	h := newTestHandler()

	body, _ := json.Marshal(loginRequest{Email: "a@b.com", Password: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "email and password are required")
}

func TestLogin_InvalidJSON(t *testing.T) {
	h := newTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader([]byte("{bad")))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid request body")
}

// --- Refresh tests (JWT parsing, no DB needed) ---

func TestRefresh_ValidRefreshToken(t *testing.T) {
	h := newTestHandler()
	userID := uuid.New()

	refreshToken := makeToken(t, jwt.MapClaims{
		"user_id": userID.String(),
		"type":    "refresh",
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(7 * 24 * time.Hour).Unix(),
	}, testSecret)

	body, _ := json.Marshal(map[string]string{"refresh_token": refreshToken})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Refresh(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp tokenResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.NotEmpty(t, resp.RefreshToken)

	// Verify the new access token has the correct user_id and type.
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(resp.Token, claims, func(t *jwt.Token) (any, error) {
		return []byte(testSecret), nil
	})
	require.NoError(t, err)
	assert.True(t, parsed.Valid)
	assert.Equal(t, "access", claims["type"])
	assert.Equal(t, userID.String(), claims["user_id"])
}

func TestRefresh_InvalidToken(t *testing.T) {
	h := newTestHandler()

	body, _ := json.Marshal(map[string]string{"refresh_token": "invalid.token.here"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Refresh(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid refresh token")
}

func TestRefresh_WrongTokenType(t *testing.T) {
	h := newTestHandler()

	// Use an access token instead of a refresh token.
	accessToken := makeToken(t, jwt.MapClaims{
		"user_id": uuid.New().String(),
		"type":    "access",
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(time.Hour).Unix(),
	}, testSecret)

	body, _ := json.Marshal(map[string]string{"refresh_token": accessToken})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Refresh(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid token type")
}

func TestRefresh_EmptyRefreshToken(t *testing.T) {
	h := newTestHandler()

	body, _ := json.Marshal(map[string]string{"refresh_token": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Refresh(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "refresh_token is required")
}

func TestRefresh_ExpiredToken(t *testing.T) {
	h := newTestHandler()

	token := makeToken(t, jwt.MapClaims{
		"user_id": uuid.New().String(),
		"type":    "refresh",
		"iat":     time.Now().Add(-2 * time.Hour).Unix(),
		"exp":     time.Now().Add(-1 * time.Hour).Unix(),
	}, testSecret)

	body, _ := json.Marshal(map[string]string{"refresh_token": token})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Refresh(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid refresh token")
}

// --- generateTokenPair test ---

func TestGenerateTokenPair(t *testing.T) {
	h := newTestHandler()
	userID := uuid.New()

	access, refresh, err := h.generateTokenPair(userID)
	require.NoError(t, err)
	assert.NotEmpty(t, access)
	assert.NotEmpty(t, refresh)

	// Parse access token.
	accessClaims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(access, accessClaims, func(t *jwt.Token) (any, error) {
		return []byte(testSecret), nil
	})
	require.NoError(t, err)
	assert.Equal(t, "access", accessClaims["type"])
	assert.Equal(t, userID.String(), accessClaims["user_id"])

	// Parse refresh token.
	refreshClaims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(refresh, refreshClaims, func(t *jwt.Token) (any, error) {
		return []byte(testSecret), nil
	})
	require.NoError(t, err)
	assert.Equal(t, "refresh", refreshClaims["type"])
	assert.Equal(t, userID.String(), refreshClaims["user_id"])
}
