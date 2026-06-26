package enrichment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchPage_ReturnsBodyAndSetsUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("<html>hi</html>"))
	}))
	defer srv.Close()

	body, err := fetchPage(context.Background(), srv.Client(), srv.URL, 1<<20)
	require.NoError(t, err)
	assert.Equal(t, "<html>hi</html>", body)
	assert.NotEmpty(t, gotUA, "a User-Agent must be set")
}

func TestFetchPage_ErrorsOn404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := fetchPage(context.Background(), srv.Client(), srv.URL, 1<<20)
	assert.Error(t, err)
}

func TestFetchPage_TruncatesToMaxBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", 10_000)))
	}))
	defer srv.Close()

	body, err := fetchPage(context.Background(), srv.Client(), srv.URL, 100)
	require.NoError(t, err)
	assert.Len(t, body, 100, "body is capped at maxBytes")
}
