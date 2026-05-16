package inbox

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInboxLead_NormalizesEmailAddress(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"uppercase + whitespace", "  ALICE@Example.COM  ", "alice@example.com"},
		{"already canonical", "alice@example.com", "alice@example.com"},
		{"mixed case", "Alice@Acme.Com", "alice@acme.com"},
		{"tab and newline", "\talice@acme.com\n", "alice@acme.com"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw := c.raw
			lead, err := NewInboxLead(uuid.New(), ChannelEmail, "Alice", "Acme", "hi", nil, &raw)
			require.NoError(t, err)
			require.NotNil(t, lead.EmailAddress)
			assert.Equal(t, c.want, *lead.EmailAddress)
		})
	}
}

func TestNewInboxLead_NilEmailAddressStaysNil(t *testing.T) {
	chatID := int64(12345)
	lead, err := NewInboxLead(uuid.New(), ChannelTelegram, "Alice", "Acme", "hi", &chatID, nil)
	require.NoError(t, err)
	assert.Nil(t, lead.EmailAddress)
}
