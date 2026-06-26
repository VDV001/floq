package domain_test

import (
	"strings"
	"testing"

	"github.com/daniil/floq/internal/enrichment/domain"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeIndustry(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"lowercases", "Fintech", "fintech"},
		{"trims", "  logistics  ", "logistics"},
		{"collapses inner whitespace", "real   estate", "real estate"},
		{"empty stays empty", "   ", ""},
		{"caps overlong garbage", strings.Repeat("x", 200), strings.Repeat("x", domain.MaxIndustryLen)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, domain.NormalizeIndustry(c.in))
		})
	}
}

func TestCompanyProfile_IsEmpty_Phase2Fields(t *testing.T) {
	assert.False(t, domain.CompanyProfile{Industry: "fintech"}.IsEmpty(),
		"a profile carrying only an industry is not empty")
	assert.False(t, domain.CompanyProfile{CompanySize: domain.CompanySizeSmall}.IsEmpty(),
		"a profile carrying only a company size is not empty")
	assert.True(t, domain.CompanyProfile{CompanySize: domain.CompanySizeUnknown}.IsEmpty(),
		"unknown size alone is still empty (no data)")
}
