package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/daniil/floq/internal/integrations/onec/domain"
)

func TestNewCredentialsConfig(t *testing.T) {
	validSecret := strings.Repeat("a", domain.WebhookSecretBytes*2) // 64 lowercase-hex chars

	tests := []struct {
		name          string
		baseURL       string
		authType      domain.AuthType
		authSecret    string
		webhookSecret string
		isActive      bool
		wantErr       error
		check         func(t *testing.T, c *domain.CredentialsConfig)
	}{
		{
			name: "empty config is valid (defaults for a new user)",
			check: func(t *testing.T, c *domain.CredentialsConfig) {
				if c.BaseURL != "" || c.IsActive {
					t.Fatalf("expected empty inactive config, got %+v", c)
				}
				if c.AuthType != domain.AuthTypeBasic {
					t.Fatalf("empty auth type must default to basic, got %q", c.AuthType)
				}
			},
		},
		{
			name:     "active requires a base url",
			isActive: true,
			authType: domain.AuthTypeBasic,
			wantErr:  domain.ErrActiveRequiresBaseURL,
		},
		{
			name:     "active with base url is ok",
			baseURL:  "https://1c.example.com",
			authType: domain.AuthTypeToken,
			isActive: true,
		},
		{
			name:     "invalid auth type rejected",
			baseURL:  "https://1c.example.com",
			authType: domain.AuthType("weird"),
			wantErr:  domain.ErrInvalidAuthType,
		},
		{
			name:     "empty auth type defaults to basic",
			baseURL:  "https://1c.example.com",
			authType: "",
			check: func(t *testing.T, c *domain.CredentialsConfig) {
				if c.AuthType != domain.AuthTypeBasic {
					t.Fatalf("want basic, got %q", c.AuthType)
				}
			},
		},
		{
			name:     "trailing slash and spaces trimmed from base url",
			baseURL:  "  https://1c.example.com/  ",
			authType: domain.AuthTypeBasic,
			check: func(t *testing.T, c *domain.CredentialsConfig) {
				if c.BaseURL != "https://1c.example.com" {
					t.Fatalf("base url not normalised: %q", c.BaseURL)
				}
			},
		},
		{
			name:          "malformed webhook secret rejected",
			baseURL:       "https://1c.example.com",
			authType:      domain.AuthTypeBasic,
			webhookSecret: "too-short",
			wantErr:       domain.ErrInvalidWebhookSecretFormat,
		},
		{
			name:          "well-formed webhook secret accepted",
			baseURL:       "https://1c.example.com",
			authType:      domain.AuthTypeBasic,
			webhookSecret: validSecret,
			check: func(t *testing.T, c *domain.CredentialsConfig) {
				if c.WebhookSecret != validSecret {
					t.Fatalf("webhook secret not preserved")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := domain.NewCredentialsConfig(tc.baseURL, tc.authType, tc.authSecret, tc.webhookSecret, tc.isActive)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("want error %v, got %v", tc.wantErr, err)
				}
				if c != nil {
					t.Fatalf("expected nil config on error, got %+v", c)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.check != nil {
				tc.check(t, c)
			}
		})
	}
}

func TestIsValidWebhookSecretFormat(t *testing.T) {
	valid := strings.Repeat("0", domain.WebhookSecretBytes*2)
	tests := []struct {
		in   string
		want bool
	}{
		{valid, true},
		{strings.Repeat("a", domain.WebhookSecretBytes*2), true},
		{"deadbeef" + strings.Repeat("0", 56), true},
		// Full hex alphabet (0-9a-f ×4 = 64 chars). Pins every boundary of
		// the [0-9] and [a-f] range checks at once; in particular the digit
		// '9' was absent from every other valid case, letting a
		// "c <= '9'" → "c < '9'" boundary mutant survive.
		{strings.Repeat("0123456789abcdef", 4), true},
		{"", false},
		{strings.Repeat("a", domain.WebhookSecretBytes*2-1), false}, // too short
		{strings.Repeat("A", domain.WebhookSecretBytes*2), false},   // uppercase not hex.EncodeToString output
		{strings.Repeat("g", domain.WebhookSecretBytes*2), false},   // non-hex char
	}
	for _, tc := range tests {
		if got := domain.IsValidWebhookSecretFormat(tc.in); got != tc.want {
			t.Errorf("IsValidWebhookSecretFormat(%q)=%v, want %v", tc.in, got, tc.want)
		}
	}
}
