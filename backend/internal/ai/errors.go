package ai

import "errors"

// ErrNotConfigured is returned when an AI provider that requires an API key is
// selected but no key is set (neither in user settings nor in the .env
// fallback). Callers use errors.Is to detect it and surface a human,
// actionable message to the user ("connect AI in Settings") instead of an
// opaque auth failure from the upstream SDK.
var ErrNotConfigured = errors.New("ai provider not configured")
