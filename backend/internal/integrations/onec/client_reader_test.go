package onec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClient_ListEvents_Success(t *testing.T) {
	var gotMethod, gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotAuth, gotPath = r.Method, r.Header.Get("Authorization"), r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{"external_id":"doc-1","external_type":"Документ.Оплата","kind":"payment","payload":{"email":"a@b.ru"}},
			{"external_id":"doc-2","external_type":"Справочник.Контрагенты","payload":{"email":"c@d.ru"}}
		]`))
	}))
	defer srv.Close()

	events, err := fastClient().ListEvents(context.Background(),
		testCreds(t, srv.URL, domain.AuthTypeToken, "tok"))

	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, http.MethodGet, gotMethod)
	assert.Equal(t, "Bearer tok", gotAuth)
	assert.Contains(t, gotPath, "floq_events")
	assert.Equal(t, "doc-1", events[0].ExternalID)
	assert.Equal(t, "Документ.Оплата", events[0].ExternalType)
	assert.Equal(t, "payment", events[0].Kind)
	assert.JSONEq(t, `{"email":"a@b.ru"}`, string(events[0].Payload))
	assert.Equal(t, "doc-2", events[1].ExternalID)
	assert.Empty(t, events[1].Kind, "kind is optional — derived from mapping downstream")
}

func TestHTTPClient_ListEvents_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	events, err := fastClient().ListEvents(context.Background(),
		testCreds(t, srv.URL, domain.AuthTypeBasic, "s"))

	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestHTTPClient_ListEvents_RetriesThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	_, err := fastClient().ListEvents(context.Background(),
		testCreds(t, srv.URL, domain.AuthTypeBasic, "s"))

	require.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls), "transient 502 must be retried once")
}

func TestHTTPClient_ListEvents_4xxIsTerminal(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := fastClient().ListEvents(context.Background(),
		testCreds(t, srv.URL, domain.AuthTypeBasic, "s"))

	require.Error(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "4xx must not be retried")
}

// Compile-time check that the HTTP client satisfies the reader port.
var _ OneCReader = (*HTTPClient)(nil)
