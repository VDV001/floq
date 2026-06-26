package domain_test

import (
	"testing"

	"github.com/daniil/floq/internal/enrichment/domain"
	"github.com/stretchr/testify/assert"
)

func TestCompanySize_IsValid(t *testing.T) {
	cases := []struct {
		name string
		size domain.CompanySize
		want bool
	}{
		{"unknown (zero value) is valid", domain.CompanySizeUnknown, true},
		{"solo", domain.CompanySizeSolo, true},
		{"small", domain.CompanySizeSmall, true},
		{"medium", domain.CompanySizeMedium, true},
		{"large", domain.CompanySizeLarge, true},
		{"enterprise", domain.CompanySizeEnterprise, true},
		{"garbage rejected", domain.CompanySize("huge"), false},
		{"numeric rejected", domain.CompanySize("500"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.size.IsValid())
		})
	}
}

func TestCompanySize_Constants(t *testing.T) {
	// The zero value MUST be the unknown bucket so an un-enriched profile is
	// never accidentally classified.
	assert.Equal(t, domain.CompanySizeUnknown, domain.CompanySize(""))
}
