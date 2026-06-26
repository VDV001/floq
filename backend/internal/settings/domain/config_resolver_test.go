package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveConfig_DBValueTakesPrecedence(t *testing.T) {
	result := ResolveConfig("from-db", "from-env")
	assert.Equal(t, "from-db", result)
}

func TestResolveConfig_FallbackWhenDBEmpty(t *testing.T) {
	result := ResolveConfig("", "from-env")
	assert.Equal(t, "from-env", result)
}

func TestResolveConfig_BothEmpty(t *testing.T) {
	result := ResolveConfig("", "")
	assert.Equal(t, "", result)
}

func TestIsEmailConfigured(t *testing.T) {
	tests := []struct {
		name                         string
		resend, host, user, password string
		want                         bool
	}{
		{"resend key alone", "re_x", "", "", "", true},
		{"full SMTP triple", "", "smtp.host", "u", "p", true},
		{"resend and SMTP both", "re_x", "smtp.host", "u", "p", true},
		{"SMTP missing password", "", "smtp.host", "u", "", false},
		{"SMTP missing user", "", "smtp.host", "", "p", false},
		{"SMTP host only", "", "smtp.host", "", "", false},
		{"nothing", "", "", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsEmailConfigured(tt.resend, tt.host, tt.user, tt.password))
		})
	}
}
