package settings

import (
	"testing"

	"github.com/daniil/floq/internal/settings/domain"
	"github.com/stretchr/testify/assert"
)

func TestMaskSecret_Empty(t *testing.T) {
	assert.Equal(t, "", maskSecret(""))
}

func TestMaskSecret_ShortString(t *testing.T) {
	// len <= 4: returns "..." + entire string
	assert.Equal(t, "...a", maskSecret("a"))
	assert.Equal(t, "...ab", maskSecret("ab"))
	assert.Equal(t, "...abc", maskSecret("abc"))
	assert.Equal(t, "...abcd", maskSecret("abcd"))
}

func TestMaskSecret_NormalString(t *testing.T) {
	// len > 4: returns "..." + last 4 chars
	assert.Equal(t, "...efgh", maskSecret("abcdefgh"))
	assert.Equal(t, "...cret", maskSecret("my-secret"))
	assert.Equal(t, "...bcde", maskSecret("abcde"))
}

func TestMaskSecret_ExactlyFiveChars(t *testing.T) {
	assert.Equal(t, "...2345", maskSecret("12345"))
}

func TestDomainToDTO_ComputedFields_IMAPActive(t *testing.T) {
	tests := []struct {
		name     string
		ds       domain.Settings
		expected bool
	}{
		{
			name:     "all IMAP fields set",
			ds:       domain.Settings{IMAPHost: "imap.gmail.com", IMAPUser: "user", IMAPPassword: "pass"},
			expected: true,
		},
		{
			name:     "missing host",
			ds:       domain.Settings{IMAPHost: "", IMAPUser: "user", IMAPPassword: "pass"},
			expected: false,
		},
		{
			name:     "missing user",
			ds:       domain.Settings{IMAPHost: "imap.gmail.com", IMAPUser: "", IMAPPassword: "pass"},
			expected: false,
		},
		{
			name:     "missing password",
			ds:       domain.Settings{IMAPHost: "imap.gmail.com", IMAPUser: "user", IMAPPassword: ""},
			expected: false,
		},
		{
			name:     "all empty",
			ds:       domain.Settings{},
			expected: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dto := domainToDTO(&tc.ds)
			assert.Equal(t, tc.expected, dto.IMAPActive)
		})
	}
}

func TestDomainToDTO_ComputedFields_ResendActive(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{"with key", "re_abc123", true},
		{"empty key", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dto := domainToDTO(&domain.Settings{ResendAPIKey: tc.key})
			assert.Equal(t, tc.expected, dto.ResendActive)
		})
	}
}

func TestDomainToDTO_ComputedFields_AIActive(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		apiKey   string
		expected bool
	}{
		{"ollama without key", "ollama", "", true},
		{"ollama with key", "ollama", "some-key", true},
		{"openai with key", "openai", "sk-abc", true},
		{"openai without key", "openai", "", false},
		{"anthropic with key", "anthropic", "sk-ant-abc", true},
		{"anthropic without key", "anthropic", "", false},
		{"no provider no key", "", "", false},
		{"no provider with key", "", "sk-abc", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dto := domainToDTO(&domain.Settings{AIProvider: tc.provider, AIAPIKey: tc.apiKey})
			assert.Equal(t, tc.expected, dto.AIActive)
		})
	}
}

func TestDomainToDTO_FieldMapping(t *testing.T) {
	ds := &domain.Settings{
		FullName:           "John Doe",
		Email:              "john@example.com",
		TelegramBotToken:   "masked-token",
		TelegramBotActive:  true,
		IMAPHost:           "imap.gmail.com",
		IMAPPort:           "993",
		IMAPUser:           "john@gmail.com",
		IMAPPassword:       "secret",
		ResendAPIKey:       "re_key",
		AIProvider:         "openai",
		AIModel:            "gpt-4o",
		AIAPIKey:           "sk-test",
		NotifyTelegram:     true,
		NotifyEmailDigest:  false,
		AutoQualify:        true,
		AutoDraft:          false,
		AutoSend:           true,
		AutoSendDelayMin:   30,
		AutoFollowup:       true,
		AutoFollowupDays:   3,
		AutoProspectToLead: true,
		AutoVerifyImport:   false,
	}

	dto := domainToDTO(ds)

	assert.Equal(t, "John Doe", dto.FullName)
	assert.Equal(t, "john@example.com", dto.Email)
	assert.Equal(t, "masked-token", dto.TelegramBotToken)
	assert.True(t, dto.TelegramBotActive)
	assert.Equal(t, "imap.gmail.com", dto.IMAPHost)
	assert.Equal(t, "993", dto.IMAPPort)
	assert.Equal(t, "john@gmail.com", dto.IMAPUser)
	assert.Equal(t, "secret", dto.IMAPPassword)
	assert.Equal(t, "re_key", dto.ResendAPIKey)
	assert.Equal(t, "openai", dto.AIProvider)
	assert.Equal(t, "gpt-4o", dto.AIModel)
	assert.Equal(t, "sk-test", dto.AIAPIKey)
	assert.True(t, dto.NotifyTelegram)
	assert.False(t, dto.NotifyEmailDigest)
	assert.True(t, dto.AutoQualify)
	assert.False(t, dto.AutoDraft)
	assert.True(t, dto.AutoSend)
	assert.Equal(t, 30, dto.AutoSendDelayMin)
	assert.True(t, dto.AutoFollowup)
	assert.Equal(t, 3, dto.AutoFollowupDays)
	assert.True(t, dto.AutoProspectToLead)
	assert.False(t, dto.AutoVerifyImport)

	// Computed fields
	assert.True(t, dto.IMAPActive)
	assert.True(t, dto.ResendActive)
	assert.True(t, dto.AIActive)
}

func TestValidateTelegramToken_IsCallable(t *testing.T) {
	// Just verify the function exists and has the expected signature.
	// We don't call it with a real token since it makes an HTTP call.
	var fn func(string) error = validateTelegramToken
	assert.NotNil(t, fn)
}
