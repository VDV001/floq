package domain

import (
	"errors"
	"strings"
)

// WebhookSecretBytes is the entropy of a webhook secret before hex-encoding.
// 32 bytes (256 bits) is well past brute-force range for the sole credential
// authenticating inbound 1C webhooks. The stored/wire form is the hex string,
// so its length is twice this.
const WebhookSecretBytes = 32

// Config-layer domain errors (declared as vars so callers can errors.Is them).
var (
	// ErrActiveRequiresBaseURL rejects activating the integration with no 1C
	// endpoint to talk to — an active-but-blank config is meaningless and would
	// silently no-op every outbound push and reconcile pass.
	ErrActiveRequiresBaseURL = errors.New("onec: cannot activate without a base url")
	// ErrInvalidWebhookSecretFormat guards a stored/incoming webhook secret that
	// is not the expected hex shape — defence against a hand-edited row.
	ErrInvalidWebhookSecretFormat = errors.New("onec: invalid webhook secret format")
)

// CredentialsConfig is the editable per-user 1C connection as managed from the
// settings UI (#110). Unlike OutboundCredentials — the read-model the outbound
// flow consumes, which demands a usable endpoint — this VO represents a config
// that may still be partial: a new user has everything blank and IsActive
// false, which is valid. The single hard invariant is that you cannot activate
// without a base URL. AuthType defaults to basic (matching the onec_credentials
// DEFAULT and its CHECK, which forbids an empty string).
type CredentialsConfig struct {
	BaseURL       string
	AuthType      AuthType
	AuthSecret    string
	WebhookSecret string
	IsActive      bool
}

// NewCredentialsConfig validates and normalises an editable 1C config. BaseURL
// is trimmed of spaces and a trailing slash (predictable path joins, mirroring
// NewOutboundCredentials). An empty auth type defaults to basic; a non-empty
// one must be valid. A non-empty webhook secret must be well-formed.
func NewCredentialsConfig(baseURL string, authType AuthType, authSecret, webhookSecret string, isActive bool) (*CredentialsConfig, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if authType == "" {
		authType = AuthTypeBasic
	}
	if !authType.IsValid() {
		return nil, ErrInvalidAuthType
	}
	if isActive && baseURL == "" {
		return nil, ErrActiveRequiresBaseURL
	}
	if webhookSecret != "" && !IsValidWebhookSecretFormat(webhookSecret) {
		return nil, ErrInvalidWebhookSecretFormat
	}
	return &CredentialsConfig{
		BaseURL:       baseURL,
		AuthType:      authType,
		AuthSecret:    authSecret,
		WebhookSecret: webhookSecret,
		IsActive:      isActive,
	}, nil
}

// IsValidWebhookSecretFormat reports whether s is the hex encoding of
// WebhookSecretBytes random bytes: exactly 2×WebhookSecretBytes lowercase hex
// characters (the shape hex.EncodeToString produces).
func IsValidWebhookSecretFormat(s string) bool {
	if len(s) != WebhookSecretBytes*2 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
