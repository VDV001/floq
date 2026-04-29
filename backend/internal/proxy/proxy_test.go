package proxy

import (
	"testing"
)

func TestNewFromURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantErr     bool
		errContains string
		wantDialer  bool
	}{
		{
			name:       "empty URL returns no error, nil dialer",
			url:        "",
			wantErr:    false,
			wantDialer: false,
		},
		{
			name:        "invalid scheme returns error",
			url:         "ftp://proxy.example.com:1080",
			wantErr:     true,
			errContains: "unsupported proxy scheme",
		},
		{
			name:       "SOCKS5 returns non-nil dialer",
			url:        "socks5://proxy.example.com:1080",
			wantErr:    false,
			wantDialer: true,
		},
		{
			name:       "HTTP proxy returns nil dialer",
			url:        "http://proxy.example.com:8080",
			wantErr:    false,
			wantDialer: false,
		},
		{
			name:       "HTTPS proxy returns nil dialer",
			url:        "https://proxy.example.com:8080",
			wantErr:    false,
			wantDialer: false,
		},
		{
			name:        "invalid URL returns error",
			url:         "://broken",
			wantErr:     true,
			errContains: "",
		},
		{
			name:       "SOCKS5 with auth parses user and password",
			url:        "socks5://user:pass@proxy.example.com:1080",
			wantErr:    false,
			wantDialer: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewFromURL(tt.url)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !containsStr(err.Error(), tt.errContains) {
					t.Fatalf("expected error to contain %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantDialer && p.Dialer() == nil {
				t.Fatal("expected non-nil Dialer, got nil")
			}
			if !tt.wantDialer && p.Dialer() != nil {
				t.Fatal("expected nil Dialer, got non-nil")
			}

			if p.HTTPClient() == nil {
				t.Fatal("HTTPClient() must always return non-nil")
			}
		})
	}
}

func TestHTTPClientTimeout(t *testing.T) {
	p, err := NewFromURL("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	client := p.HTTPClient()
	if client.Timeout != defaultTimeout {
		t.Fatalf("expected timeout %v, got %v", defaultTimeout, client.Timeout)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (substr == "" || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
