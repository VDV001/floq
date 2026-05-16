package domain_test

import (
	"testing"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIdentity_RequiresAtLeastOneIdentifier(t *testing.T) {
	t.Run("all empty rejected", func(t *testing.T) {
		_, err := domain.NewIdentity(uuid.New(), "", "", "")
		require.ErrorIs(t, err, domain.ErrIdentityNoIdentifiers)
	})

	t.Run("whitespace-only rejected after normalization", func(t *testing.T) {
		_, err := domain.NewIdentity(uuid.New(), "  ", "()- ", "  @  ")
		require.ErrorIs(t, err, domain.ErrIdentityNoIdentifiers)
	})
}

func TestNewIdentity_NormalizesIdentifiers(t *testing.T) {
	cases := []struct {
		name      string
		email     string
		phone     string
		tg        string
		wantEmail string
		wantPhone string
		wantTg    string
	}{
		{
			name:      "all canonical",
			email:     "alice@acme.com",
			phone:     "+79991234567",
			tg:        "alice_bot",
			wantEmail: "alice@acme.com",
			wantPhone: "+79991234567",
			wantTg:    "alice_bot",
		},
		{
			name:      "email uppercased and padded",
			email:     "  ALICE@Acme.COM  ",
			wantEmail: "alice@acme.com",
		},
		{
			name:      "phone with separators",
			phone:     "+7 (999) 123-45-67",
			wantPhone: "+79991234567",
		},
		{
			name:   "tg with leading @ and uppercase",
			tg:     "@ALICE_BOT",
			wantTg: "alice_bot",
		},
		{
			name:      "all three normalized in one go",
			email:     "ALICE@Acme.COM",
			phone:     "+7 999 123-45-67",
			tg:        "@Alice",
			wantEmail: "alice@acme.com",
			wantPhone: "+79991234567",
			wantTg:    "alice",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, err := domain.NewIdentity(uuid.New(), c.email, c.phone, c.tg)
			require.NoError(t, err)
			assert.Equal(t, c.wantEmail, id.Email)
			assert.Equal(t, c.wantPhone, id.Phone)
			assert.Equal(t, c.wantTg, id.TelegramUsername)
		})
	}
}

func TestNewIdentity_AssignsIDAndTimestamp(t *testing.T) {
	userID := uuid.New()
	id, err := domain.NewIdentity(userID, "alice@acme.com", "", "")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, id.ID)
	assert.Equal(t, userID, id.UserID)
	assert.False(t, id.CreatedAt.IsZero(), "CreatedAt must be set by the factory")
}
