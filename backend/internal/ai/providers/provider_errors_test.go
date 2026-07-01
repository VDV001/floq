package providers

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyProviderStatus(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   error
	}{
		{"unauthorized", http.StatusUnauthorized, ErrProviderAuth},
		{"forbidden", http.StatusForbidden, ErrProviderAuth},
		{"too many requests", http.StatusTooManyRequests, ErrProviderRateLimit},
		{"server error", http.StatusInternalServerError, ErrProviderUnreachable},
		{"bad gateway", http.StatusBadGateway, ErrProviderUnreachable},
		{"no status", 0, ErrProviderUnreachable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, errors.Is(classifyProviderStatus(tc.status), tc.want),
				"status %d must classify as %v; got %v", tc.status, tc.want, classifyProviderStatus(tc.status))
		})
	}
}
