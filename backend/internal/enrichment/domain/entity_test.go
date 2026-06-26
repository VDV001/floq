package domain_test

import (
	"errors"
	"testing"

	"github.com/daniil/floq/internal/enrichment/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDomain(t *testing.T) {
	cases := []struct {
		name    string
		email   string
		want    string
		wantErr error
	}{
		{"corporate lowercases", "ivan@Acme.RU", "acme.ru", nil},
		{"strips www", "sales@www.acme.ru", "acme.ru", nil},
		{"trims spaces", "  ivan@acme.ru  ", "acme.ru", nil},
		{"subdomain kept", "ivan@team.acme.ru", "team.acme.ru", nil},
		{"free gmail", "ivan@gmail.com", "", domain.ErrFreeEmailProvider},
		{"free yandex", "ivan@yandex.ru", "", domain.ErrFreeEmailProvider},
		{"free mail.ru", "ivan@mail.ru", "", domain.ErrFreeEmailProvider},
		{"free outlook", "ivan@outlook.com", "", domain.ErrFreeEmailProvider},
		{"empty", "", "", domain.ErrInvalidDomain},
		{"no at", "not-an-email", "", domain.ErrInvalidDomain},
		{"no domain part", "ivan@", "", domain.ErrInvalidDomain},
		{"no local part", "@acme.ru", "", domain.ErrInvalidDomain},
		{"no dot in domain", "ivan@localhost", "", domain.ErrInvalidDomain},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, err := domain.NewDomain(c.email)
			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, d.String())
		})
	}
}

func TestStatus_IsValid(t *testing.T) {
	assert.True(t, domain.StatusPending.IsValid())
	assert.True(t, domain.StatusEnriched.IsValid())
	assert.True(t, domain.StatusFailed.IsValid())
	assert.False(t, domain.Status("bogus").IsValid())
}

func TestNewPendingEnrichment(t *testing.T) {
	userID := uuid.New()
	d, err := domain.NewDomain("ivan@acme.ru")
	require.NoError(t, err)

	e, err := domain.NewPendingEnrichment(userID, d)
	require.NoError(t, err)
	assert.Equal(t, userID, e.UserID)
	assert.Equal(t, "acme.ru", e.Domain.String())
	assert.Equal(t, domain.StatusPending, e.Status)
	assert.Equal(t, 0, e.Attempts)
	assert.Nil(t, e.EnrichedAt)
}

func TestNewPendingEnrichment_RejectsNilUser(t *testing.T) {
	d, _ := domain.NewDomain("ivan@acme.ru")
	_, err := domain.NewPendingEnrichment(uuid.Nil, d)
	assert.Error(t, err)
}

func TestCompanyEnrichment_MarkEnriched(t *testing.T) {
	userID := uuid.New()
	d, _ := domain.NewDomain("ivan@acme.ru")
	e, _ := domain.NewPendingEnrichment(userID, d)

	profile := domain.CompanyProfile{
		Title:       "Acme LLC",
		Description: "We make widgets",
		Emails:      []string{"info@acme.ru"},
		Socials:     []string{"https://t.me/acme"},
	}
	e.MarkEnriched(profile, 7*24*60*60) // ttlSeconds

	assert.Equal(t, domain.StatusEnriched, e.Status)
	assert.Equal(t, profile, e.Profile)
	assert.Empty(t, e.Error)
	require.NotNil(t, e.EnrichedAt)
	require.NotNil(t, e.ExpiresAt)
	assert.True(t, e.ExpiresAt.After(*e.EnrichedAt))
}

func TestCompanyEnrichment_MarkFailed(t *testing.T) {
	userID := uuid.New()
	d, _ := domain.NewDomain("ivan@acme.ru")
	e, _ := domain.NewPendingEnrichment(userID, d)

	e.MarkFailed("fetch timeout")
	assert.Equal(t, domain.StatusFailed, e.Status)
	assert.Equal(t, "fetch timeout", e.Error)
	assert.Equal(t, 1, e.Attempts)

	e.MarkFailed("again")
	assert.Equal(t, 2, e.Attempts, "attempts accumulate across failures")
}

func TestCompanyProfile_IsEmpty(t *testing.T) {
	assert.True(t, domain.CompanyProfile{}.IsEmpty())
	assert.False(t, domain.CompanyProfile{Title: "Acme"}.IsEmpty())
	assert.False(t, domain.CompanyProfile{Emails: []string{"a@b.ru"}}.IsEmpty())
}

func TestDomainErrors_AreDistinct(t *testing.T) {
	assert.False(t, errors.Is(domain.ErrFreeEmailProvider, domain.ErrInvalidDomain))
}
