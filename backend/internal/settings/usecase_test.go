package settings

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/settings/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateSettings_VerifiedFlags pins the honest-«Готово» wiring (#222):
// a channel's *_verified flag is set from an explicit client field (sent
// right after a passing connection test) and is cleared whenever the
// channel's credentials change without a fresh verification.
func TestUpdateSettings_VerifiedFlags(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		field     string
		wantSet   bool // field present in the update at all
		wantValue bool
	}{
		{"ai_verified true persists", `{"ai_provider":"openai","ai_model":"gpt-4o","ai_api_key":"sk","ai_verified":true}`, "ai_verified", true, true},
		{"ai creds change without verify clears", `{"ai_api_key":"sk-new"}`, "ai_verified", true, false},
		{"ai model change clears", `{"ai_model":"gpt-4o-mini"}`, "ai_verified", true, false},
		{"smtp verified true persists", `{"smtp_host":"h","smtp_user":"u","smtp_password":"p","smtp_verified":true}`, "smtp_verified", true, true},
		{"smtp creds change clears", `{"smtp_password":"new"}`, "smtp_verified", true, false},
		{"imap verified true persists", `{"imap_host":"h","imap_user":"u","imap_verified":true}`, "imap_verified", true, true},
		{"imap creds change clears", `{"imap_host":"other"}`, "imap_verified", true, false},
		{"unrelated update leaves ai_verified untouched", `{"notify_telegram":true}`, "ai_verified", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMockSettingsRepo()
			uc := NewUseCase(repo, &mockTelegramValidator{})
			raw, input := parseSettingsBody([]byte(tc.body))
			_, err := uc.UpdateSettings(context.Background(), uuid.New(), raw, input)
			require.NoError(t, err)
			v, ok := repo.updated[tc.field]
			assert.Equal(t, tc.wantSet, ok, "field %q presence in update", tc.field)
			if tc.wantSet {
				assert.Equal(t, tc.wantValue, v, "field %q value", tc.field)
			}
		})
	}
}

func TestMaskSecret_Empty(t *testing.T) {
	assert.Equal(t, "", maskSecret(""))
}

func TestMaskSecret_ShortString(t *testing.T) {
	// len <= 4: too short to mask meaningfully → replaced wholesale, never
	// leaked verbatim (matches the onec package's maskSecret).
	assert.Equal(t, "••••", maskSecret("a"))
	assert.Equal(t, "••••", maskSecret("ab"))
	assert.Equal(t, "••••", maskSecret("abc"))
	assert.Equal(t, "••••", maskSecret("abcd"))
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

// #222: the *Active DTO flags reflect a PASSED connection test
// (*Verified), not merely "fields present". "Filled but unverified" must
// read as NOT active — that was the false-«Готово» bug.

func TestDomainToDTO_ComputedFields_IMAPActive(t *testing.T) {
	tests := []struct {
		name     string
		ds       domain.Settings
		expected bool
	}{
		{"verified", domain.Settings{IMAPVerified: true}, true},
		{"filled but unverified", domain.Settings{IMAPHost: "imap.gmail.com", IMAPUser: "user", IMAPPassword: "pass"}, false},
		{"verified even if fields look empty (test used stored creds)", domain.Settings{IMAPVerified: true, IMAPHost: ""}, true},
		{"all empty", domain.Settings{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, domainToDTO(&tc.ds).IMAPActive)
		})
	}
}

func TestDomainToDTO_ComputedFields_SMTPActive(t *testing.T) {
	tests := []struct {
		name     string
		ds       domain.Settings
		expected bool
	}{
		{"verified", domain.Settings{SMTPVerified: true}, true},
		{"filled but unverified", domain.Settings{SMTPHost: "smtp.x", SMTPUser: "u", SMTPPassword: "p"}, false},
		{"all empty", domain.Settings{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, domainToDTO(&tc.ds).SMTPActive)
		})
	}
}

func TestDomainToDTO_ComputedFields_ResendActive(t *testing.T) {
	// #241: like the other channels (#222), ResendActive must reflect a
	// PASSED connection test (ResendVerified), not merely a key present.
	tests := []struct {
		name     string
		ds       domain.Settings
		expected bool
	}{
		{"verified", domain.Settings{ResendVerified: true}, true},
		{"key present but unverified", domain.Settings{ResendAPIKey: "re_abc123"}, false},
		{"nothing set", domain.Settings{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, domainToDTO(&tc.ds).ResendActive)
		})
	}
}

func TestDomainToDTO_ComputedFields_AIActive(t *testing.T) {
	tests := []struct {
		name     string
		ds       domain.Settings
		expected bool
	}{
		{"verified", domain.Settings{AIVerified: true}, true},
		{"ollama unverified (the reported bug)", domain.Settings{AIProvider: "ollama"}, false},
		{"cloud key present but unverified", domain.Settings{AIProvider: "openai", AIAPIKey: "sk-abc"}, false},
		{"verified ollama (passed /api/tags ping)", domain.Settings{AIProvider: "ollama", AIVerified: true}, true},
		{"nothing set", domain.Settings{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, domainToDTO(&tc.ds).AIActive)
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
		IMAPVerified:       true,
		ResendAPIKey:       "re_key",
		ResendVerified:     true,
		AIProvider:         "openai",
		AIModel:            "gpt-4o",
		AIAPIKey:           "sk-test",
		AIVerified:         true,
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
	// Secrets are masked by the DTO mapping (last 4 chars).
	assert.Equal(t, "...oken", dto.TelegramBotToken)
	assert.True(t, dto.TelegramBotActive)
	assert.Equal(t, "imap.gmail.com", dto.IMAPHost)
	assert.Equal(t, "993", dto.IMAPPort)
	assert.Equal(t, "john@gmail.com", dto.IMAPUser)
	assert.Equal(t, "...cret", dto.IMAPPassword)
	assert.Equal(t, "..._key", dto.ResendAPIKey)
	assert.Equal(t, "openai", dto.AIProvider)
	assert.Equal(t, "gpt-4o", dto.AIModel)
	assert.Equal(t, "...test", dto.AIAPIKey)
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

func TestHTTPTelegramValidator_IsCallable(t *testing.T) {
	// Just verify the type exists and implements the interface.
	v := &HTTPTelegramValidator{}
	assert.NotNil(t, v)
}
