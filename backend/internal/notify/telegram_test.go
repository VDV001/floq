package notify

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTelegramNotifier_InvalidToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{name: "empty token", token: ""},
		{name: "garbage token", token: "not-a-real-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier, err := NewTelegramNotifier(tt.token, 123)
			require.Error(t, err)
			assert.Nil(t, notifier)
		})
	}
}
