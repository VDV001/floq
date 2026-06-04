package onec

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClient_TestConnection(t *testing.T) {
	t.Run("2xx returns nil and sends auth", func(t *testing.T) {
		var gotMethod, gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		creds := testCreds(t, srv.URL, domain.AuthTypeToken, "tok123")
		err := fastClient().TestConnection(context.Background(), creds)
		require.NoError(t, err)
		assert.Equal(t, http.MethodGet, gotMethod)
		assert.Equal(t, "Bearer tok123", gotAuth)
	})

	t.Run("401 maps to ErrOnecAuth", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		err := fastClient().TestConnection(context.Background(), testCreds(t, srv.URL, domain.AuthTypeBasic, "x"))
		assert.True(t, errors.Is(err, ErrOnecAuth), "got %v", err)
	})

	t.Run("403 maps to ErrOnecAuth", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()
		err := fastClient().TestConnection(context.Background(), testCreds(t, srv.URL, domain.AuthTypeBasic, "x"))
		assert.True(t, errors.Is(err, ErrOnecAuth), "got %v", err)
	})

	t.Run("500 maps to ErrOnecBadResponse", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		err := fastClient().TestConnection(context.Background(), testCreds(t, srv.URL, domain.AuthTypeBasic, "x"))
		assert.True(t, errors.Is(err, ErrOnecBadResponse), "got %v", err)
	})

	t.Run("dial failure maps to ErrOnecUnreachable", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
		url := srv.URL
		srv.Close() // server gone → connection refused
		err := fastClient().TestConnection(context.Background(), testCreds(t, url, domain.AuthTypeBasic, "x"))
		assert.True(t, errors.Is(err, ErrOnecUnreachable), "got %v", err)
	})
}

// Compile-time check: HTTPClient satisfies the ConnectionTester port.
var _ ConnectionTester = (*HTTPClient)(nil)
