package domain_test

import (
	"testing"

	"github.com/daniil/floq/internal/enrichment/domain"
	"github.com/stretchr/testify/assert"
)

func TestLegalDetails_IsEmpty(t *testing.T) {
	assert.True(t, domain.LegalDetails{}.IsEmpty())
	assert.False(t, domain.LegalDetails{INN: "7707083893"}.IsEmpty())
	assert.False(t, domain.LegalDetails{Address: "Москва"}.IsEmpty())
}

func TestCompanyProfile_IsEmpty_Legal(t *testing.T) {
	assert.False(t, domain.CompanyProfile{Legal: domain.LegalDetails{INN: "7707083893"}}.IsEmpty(),
		"a profile carrying only legal details is not empty")
	assert.True(t, domain.CompanyProfile{Legal: domain.LegalDetails{}}.IsEmpty(),
		"empty legal details alone is still empty")
}
