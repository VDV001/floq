package providers

import "errors"

// Shared health-probe sentinels for API-backed providers (OpenAI, Groq,
// Claude). CheckHealth classifies the probe's HTTP status into one of
// these; the composition root maps them to the settings vocabulary via
// errors.Is, which in turn maps to user-facing copy.
var (
	// ErrProviderAuth — the provider rejected the API key (401/403).
	ErrProviderAuth = errors.New("provider auth rejected")

	// ErrProviderRateLimit — the provider throttled the probe (429).
	ErrProviderRateLimit = errors.New("provider rate limited")

	// ErrProviderUnreachable — transport failure, 5xx, or any other
	// non-success status from the health probe.
	ErrProviderUnreachable = errors.New("provider unreachable")
)

// classifyProviderStatus maps an HTTP status from a provider auth probe
// to a shared sentinel: 401/403 → auth, 429 → rate limit, everything
// else → unreachable.
func classifyProviderStatus(_ int) error {
	// RED stub — real mapping lands in the GREEN commit.
	return ErrProviderUnreachable
}
