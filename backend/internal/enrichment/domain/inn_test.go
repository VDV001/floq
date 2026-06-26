package domain_test

import (
	"testing"

	"github.com/daniil/floq/internal/enrichment/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewINN(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"valid 10-digit legal entity (Sberbank)", "7707083893", false},
		{"valid 12-digit sole proprietor", "500100732259", false},
		{"trims spaces", "  7707083893  ", false},
		{"bad 10-digit control digit", "7707083890", true},
		{"bad 12-digit control digit", "500100732250", true},
		{"wrong length 11", "77070838931", true},
		{"too short", "770708", true},
		{"empty", "", true},
		{"non-digits", "77070838AB", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inn, err := domain.NewINN(c.in)
			if c.wantErr {
				assert.ErrorIs(t, err, domain.ErrInvalidINN)
				return
			}
			require.NoError(t, err)
			// String returns the normalized (trimmed) digits.
			assert.NotEmpty(t, inn.String())
		})
	}
}

func TestINN_StringNormalized(t *testing.T) {
	inn, err := domain.NewINN("  7707083893 ")
	require.NoError(t, err)
	assert.Equal(t, "7707083893", inn.String())
}
