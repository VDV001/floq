package enrichment

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsBlockedIP(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true},   // loopback
		{"::1", true},         // loopback v6
		{"10.0.0.5", true},    // RFC1918
		{"172.16.0.1", true},  // RFC1918
		{"192.168.1.1", true}, // RFC1918
		{"169.254.169.254", true}, // link-local (cloud metadata)
		{"0.0.0.0", true},     // unspecified
		{"fc00::1", true},     // ULA
		{"8.8.8.8", false},    // public
		{"1.1.1.1", false},    // public
	}
	for _, c := range cases {
		t.Run(c.ip, func(t *testing.T) {
			assert.Equal(t, c.blocked, isBlockedIP(net.ParseIP(c.ip)))
		})
	}
}

func TestGuardedClient_BlocksLoopbackEgress(t *testing.T) {
	// httptest listens on 127.0.0.1 — the guarded client's dialer must refuse
	// to connect to it (SSRF defense layer 2), even though the URL is "valid".
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>internal</html>"))
	}))
	defer srv.Close()

	_, err := fetchPage(context.Background(), newGuardedClient(), srv.URL, 1<<20)
	require.Error(t, err, "guarded client must block egress to a loopback address")
}

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
