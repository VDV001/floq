package audit

import (
	"strings"
	"testing"
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
		in   string
		max  int
		want string
	}{
		{"short", 10, "short"},
		{"exactly 10", 10, "exactly 10"},
		{"this string is longer than max", 10, "this stri…"},
		{"x", 1, "x"},
		{"xy", 1, "…"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := truncate(tc.in, tc.max); got != tc.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}
