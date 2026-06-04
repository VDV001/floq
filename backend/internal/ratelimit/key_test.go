package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daniil/floq/internal/ratelimit"
)

func TestIPFromRequest(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		trustProxy bool
		headers    map[string]string
		wantIP     string
		wantOK     bool
	}{
		{
			name:       "remote addr host:port strips port",
			remoteAddr: "203.0.113.7:54321",
			wantIP:     "203.0.113.7",
			wantOK:     true,
		},
		{
			name:       "remote addr already bare ip",
			remoteAddr: "203.0.113.7",
			wantIP:     "203.0.113.7",
			wantOK:     true,
		},
		{
			name:       "ipv6 remote addr strips port",
			remoteAddr: "[2001:db8::1]:443",
			wantIP:     "2001:db8::1",
			wantOK:     true,
		},
		{
			name:       "unparseable remote addr fails open",
			remoteAddr: "garbage",
			wantIP:     "",
			wantOK:     false,
		},
		{
			name:       "empty remote addr and no headers fails open",
			remoteAddr: "",
			wantIP:     "",
			wantOK:     false,
		},

		// --- trustProxy=false: forwarded headers MUST be ignored
		// (otherwise a directly-exposed app lets an attacker rotate
		// X-Forwarded-For to dodge the per-IP cap). ---
		{
			name:       "untrusted proxy ignores x-forwarded-for, uses remote addr",
			remoteAddr: "203.0.113.7:54321",
			trustProxy: false,
			headers:    map[string]string{"X-Forwarded-For": "198.51.100.23"},
			wantIP:     "203.0.113.7",
			wantOK:     true,
		},
		{
			name:       "untrusted proxy ignores x-real-ip, uses remote addr",
			remoteAddr: "203.0.113.7:54321",
			trustProxy: false,
			headers:    map[string]string{"X-Real-IP": "198.51.100.23"},
			wantIP:     "203.0.113.7",
			wantOK:     true,
		},
		{
			name:       "untrusted proxy does not fall back to spoofable header",
			remoteAddr: "garbage",
			trustProxy: false,
			headers:    map[string]string{"X-Forwarded-For": "198.51.100.23"},
			wantIP:     "",
			wantOK:     false,
		},

		// --- trustProxy=true: honor headers, client is behind a proxy
		// that overwrites them with the real peer address. ---
		{
			name:       "trusted proxy: x-forwarded-for takes precedence",
			remoteAddr: "10.0.0.1:9999",
			trustProxy: true,
			headers:    map[string]string{"X-Forwarded-For": "198.51.100.23"},
			wantIP:     "198.51.100.23",
			wantOK:     true,
		},
		{
			name:       "trusted proxy: x-forwarded-for uses first ip in chain",
			remoteAddr: "10.0.0.1:9999",
			trustProxy: true,
			headers:    map[string]string{"X-Forwarded-For": "198.51.100.23, 70.41.3.18, 150.172.238.178"},
			wantIP:     "198.51.100.23",
			wantOK:     true,
		},
		{
			name:       "trusted proxy: trims surrounding whitespace",
			remoteAddr: "10.0.0.1:9999",
			trustProxy: true,
			headers:    map[string]string{"X-Forwarded-For": "  198.51.100.23  ,70.41.3.18"},
			wantIP:     "198.51.100.23",
			wantOK:     true,
		},
		{
			name:       "trusted proxy: x-real-ip used when no forwarded-for",
			remoteAddr: "10.0.0.1:9999",
			trustProxy: true,
			headers:    map[string]string{"X-Real-IP": "192.0.2.44"},
			wantIP:     "192.0.2.44",
			wantOK:     true,
		},
		{
			name:       "trusted proxy: forwarded-for wins over real-ip",
			remoteAddr: "10.0.0.1:9999",
			trustProxy: true,
			headers:    map[string]string{"X-Forwarded-For": "198.51.100.23", "X-Real-IP": "192.0.2.44"},
			wantIP:     "198.51.100.23",
			wantOK:     true,
		},
		{
			name:       "trusted proxy: empty forwarded-for falls back to remote addr",
			remoteAddr: "203.0.113.7:54321",
			trustProxy: true,
			headers:    map[string]string{"X-Forwarded-For": "   "},
			wantIP:     "203.0.113.7",
			wantOK:     true,
		},
		{
			name:       "trusted proxy: non-ip forwarded-for falls through to real-ip",
			remoteAddr: "10.0.0.1:9999",
			trustProxy: true,
			headers:    map[string]string{"X-Forwarded-For": "not-an-ip", "X-Real-IP": "192.0.2.44"},
			wantIP:     "192.0.2.44",
			wantOK:     true,
		},
		{
			name:       "trusted proxy: non-ip headers fall through to remote addr",
			remoteAddr: "203.0.113.7:54321",
			trustProxy: true,
			headers:    map[string]string{"X-Forwarded-For": "not-an-ip", "X-Real-IP": "also-bad"},
			wantIP:     "203.0.113.7",
			wantOK:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
			req.RemoteAddr = tc.remoteAddr
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			ip, ok := ratelimit.IPFromRequest(req, tc.trustProxy)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (ip=%q)", ok, tc.wantOK, ip)
			}
			if ip != tc.wantIP {
				t.Errorf("ip = %q, want %q", ip, tc.wantIP)
			}
		})
	}
}

func TestIPKeyFunc(t *testing.T) {
	keyFn := ratelimit.IPKeyFunc("ratelimit:auth-login:", false)

	t.Run("prefixes the resolved ip", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		req.RemoteAddr = "203.0.113.7:54321"

		key, ok := keyFn(req)
		if !ok {
			t.Fatal("expected ok=true for a resolvable ip")
		}
		if key != "ratelimit:auth-login:203.0.113.7" {
			t.Errorf("key = %q, want %q", key, "ratelimit:auth-login:203.0.113.7")
		}
	})

	t.Run("untrusted proxy ignores spoofed x-forwarded-for", func(t *testing.T) {
		// Two requests with the SAME peer but DIFFERENT spoofed XFF must
		// map to the SAME key — otherwise the per-IP cap is bypassable.
		reqA := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		reqA.RemoteAddr = "203.0.113.7:1111"
		reqA.Header.Set("X-Forwarded-For", "1.1.1.1")
		reqB := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		reqB.RemoteAddr = "203.0.113.7:2222"
		reqB.Header.Set("X-Forwarded-For", "2.2.2.2")

		keyA, _ := keyFn(reqA)
		keyB, _ := keyFn(reqB)
		if keyA != keyB {
			t.Errorf("spoofed XFF produced distinct keys %q vs %q — per-IP cap bypassable", keyA, keyB)
		}
	})

	t.Run("unresolvable ip bypasses (ok=false)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		req.RemoteAddr = "garbage"

		key, ok := keyFn(req)
		if ok {
			t.Errorf("expected ok=false when ip cannot be resolved, got key=%q", key)
		}
	})

	t.Run("distinct ips map to distinct keys", func(t *testing.T) {
		reqA := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		reqA.RemoteAddr = "203.0.113.7:1111"
		reqB := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		reqB.RemoteAddr = "203.0.113.8:2222"

		keyA, _ := keyFn(reqA)
		keyB, _ := keyFn(reqB)
		if keyA == keyB {
			t.Errorf("distinct ips produced identical keys %q", keyA)
		}
	})

	t.Run("trusted proxy honors x-forwarded-for", func(t *testing.T) {
		trusted := ratelimit.IPKeyFunc("ratelimit:auth-login:", true)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		req.RemoteAddr = "10.0.0.1:9999"
		req.Header.Set("X-Forwarded-For", "198.51.100.23")

		key, ok := trusted(req)
		if !ok || key != "ratelimit:auth-login:198.51.100.23" {
			t.Errorf("trusted proxy key = %q ok=%v, want ...198.51.100.23", key, ok)
		}
	})
}
