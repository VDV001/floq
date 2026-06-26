package domain_test

import (
	"testing"

	"github.com/daniil/floq/internal/enrichment/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOGRN(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"valid 13-digit OGRN (Sberbank)", "1027700132195", false},
		{"valid 15-digit OGRNIP", "304500116000157", false},
		{"trims spaces", " 1027700132195 ", false},
		{"bad 13-digit control", "1027700132190", true},
		{"bad 15-digit control", "304500116000150", true},
		{"wrong length 14", "10277001321955", true},
		{"too short", "102770", true},
		{"empty", "", true},
		{"non-digits", "102770013219X", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o, err := domain.NewOGRN(c.in)
			if c.wantErr {
				assert.ErrorIs(t, err, domain.ErrInvalidOGRN)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, o.String())
		})
	}
}

func TestOGRN_StringNormalized(t *testing.T) {
	o, err := domain.NewOGRN(" 1027700132195 ")
	require.NoError(t, err)
	assert.Equal(t, "1027700132195", o.String())
}
