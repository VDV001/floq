package analytics_test

import (
	"errors"
	"testing"

	"github.com/daniil/floq/internal/analytics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStatusFilter(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "any", false},
		{"any", "any", false},
		{"new", "new", false},
		{"qualified", "qualified", false},
		{"in_conversation", "in_conversation", false},
		{"followup", "followup", false},
		{"closed", "closed", false},
		{"lost", "", true},      // not in this schema's lead_status enum
		{"garbage", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := analytics.ParseStatusFilter(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, errors.Is(err, analytics.ErrInvalidStatusFilter))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseChannelFilter(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "any", false},
		{"any", "any", false},
		{"telegram", "telegram", false},
		{"email", "email", false},
		{"sms", "", true},
		{"garbage", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := analytics.ParseChannelFilter(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, errors.Is(err, analytics.ErrInvalidChannelFilter))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
