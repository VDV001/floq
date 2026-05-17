package audit

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeErrorMessage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		in           string
		wantNotMatch []string
		wantMatch    []string
	}{
		{
			name:         "strips email",
			in:           "openai 400: invalid prompt 'reset password for alice@acme.com'",
			wantNotMatch: []string{"alice@acme.com"},
			wantMatch:    []string{"[REDACTED]", "openai 400"},
		},
		{
			name:         "strips phone",
			in:           "twilio error contacting +1 (555) 867-5309 directly",
			wantNotMatch: []string{"5309", "867"},
			wantMatch:    []string{"[REDACTED]", "twilio error"},
		},
		{
			name:         "strips sk- key",
			in:           "openai auth failed for key sk-proj-abcdefghij1234567890ABCDEF",
			wantNotMatch: []string{"sk-proj-abcdefghij1234567890ABCDEF"},
			wantMatch:    []string{"[REDACTED]"},
		},
		{
			name:         "strips bearer token",
			in:           "401 Unauthorized: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.abc.def",
			wantNotMatch: []string{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
			wantMatch:    []string{"[REDACTED]"},
		},
		{
			name:         "strips AWS access key",
			in:           "S3 error: invalid access key AKIAIOSFODNN7EXAMPLE used by client",
			wantNotMatch: []string{"AKIAIOSFODNN7EXAMPLE"},
			wantMatch:    []string{"[REDACTED]", "S3 error"},
		},
		{
			name:         "strips raw Authorization header value",
			in:           "401: backend rejected Authorization: abcdef1234567890ABCDEF",
			wantNotMatch: []string{"abcdef1234567890ABCDEF"},
			wantMatch:    []string{"[REDACTED]"},
		},
		{
			name:         "strips basic-auth userinfo in URL",
			in:           "connection to postgres://admin:hunter2@db.internal:5432/x refused",
			wantNotMatch: []string{"admin:hunter2"},
			wantMatch:    []string{"[REDACTED]"},
		},
		{
			name:         "strips bare IPv4",
			in:           "connection from 192.168.1.42 failed during handshake",
			wantNotMatch: []string{"192.168.1.42"},
			wantMatch:    []string{"[REDACTED]"},
		},
		{
			name:         "leaves clean error untouched",
			in:           "openai 429 rate_limit_exceeded",
			wantNotMatch: []string{"[REDACTED]"},
			wantMatch:    []string{"openai 429 rate_limit_exceeded"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeErrorMessage(tc.in)
			for _, m := range tc.wantNotMatch {
				if strings.Contains(got, m) {
					t.Errorf("sanitized output still contains %q: %q", m, got)
				}
			}
			for _, m := range tc.wantMatch {
				if !strings.Contains(got, m) {
					t.Errorf("sanitized output missing %q: %q", m, got)
				}
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"short fits as-is", "short", 10, "short"},
		{"exactly at cap", "exactly 10", 10, "exactly 10"},
		// "…" is 3 bytes; max=10 leaves 7 ASCII chars before the marker.
		{"ascii cut respects byte cap", "this string is longer than max", 10, "this st…"},
		{"max too small for marker keeps ascii prefix", "xy", 1, "x"},
		{"max too small for marker drops multi-byte rune", "ы", 1, ""},
		{"max zero returns empty", "anything", 0, ""},
		{"max negative returns empty", "anything", -5, ""},
		// Multi-byte safety: Russian. Cap 10 bytes; '…' is 3 bytes.
		// Each Cyrillic letter is 2 bytes. "ош" = 4 bytes, "оши" = 6
		// bytes. Limit = 10-3 = 7; walking back to rune start lands
		// at byte 6 (start of "и"). Want "оши…" = 6 + 3 = 9 bytes.
		{"utf8 russian walks back to rune start", "ошибка авторизации", 10, "оши…"},
		// Emoji (4 bytes). Cap 6, marker 3 bytes; limit 3 lands inside
		// the emoji — walk back to 0, producing just the marker.
		{"utf8 emoji shorter than rune", "🔥abcdef", 6, "…"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := truncate(tc.in, tc.max)
			if got != tc.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("truncate produced invalid UTF-8: %q", got)
			}
		})
	}
}
