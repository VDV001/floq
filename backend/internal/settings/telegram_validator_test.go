package settings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTPTelegramValidator_Validate_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/bottest-token/getMe")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	v := &HTTPTelegramValidator{baseURL: srv.URL}
	err := v.Validate("test-token")
	assert.NoError(t, err)
}

func TestHTTPTelegramValidator_Validate_NotOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": false})
	}))
	defer srv.Close()

	v := &HTTPTelegramValidator{baseURL: srv.URL}
	err := v.Validate("bad-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ok=false")
}

func TestHTTPTelegramValidator_Validate_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	v := &HTTPTelegramValidator{baseURL: srv.URL}
	err := v.Validate("bad-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 401")
}

func TestHTTPTelegramValidator_Validate_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	v := &HTTPTelegramValidator{baseURL: srv.URL}
	err := v.Validate("token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

func TestHTTPTelegramValidator_Validate_ConnectionError(t *testing.T) {
	v := &HTTPTelegramValidator{baseURL: "http://127.0.0.1:1"}
	err := v.Validate("token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to reach")
}
