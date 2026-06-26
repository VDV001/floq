package outbound

import (
	"errors"
	"fmt"
)

// Sentinel errors for outbound sending. Wrapped errors expose these via
// errors.Is so callers (and tests) can branch on the failure mode without
// matching error message strings.
var (
	// ErrNoResendAPIKey is returned by sendViaResend when no Resend API key
	// is configured for this owner (neither in DB nor in the fallback env).
	ErrNoResendAPIKey = errors.New("no Resend API key configured")

	// ErrResendAPI is returned when the Resend API responds with a non-2xx
	// status. The concrete error is *ResendAPIError, which carries the
	// status code; ResendAPIError.Unwrap returns ErrResendAPI so
	// errors.Is(err, ErrResendAPI) reports true.
	ErrResendAPI = errors.New("resend API error")
)

// ResendAPIError wraps a non-2xx response from the Resend HTTP API.
type ResendAPIError struct {
	StatusCode int
}

func (e *ResendAPIError) Error() string {
	return fmt.Sprintf("resend API error: status %d", e.StatusCode)
}

// Unwrap allows errors.Is(err, ErrResendAPI) to match.
func (e *ResendAPIError) Unwrap() error {
	return ErrResendAPI
}

// resendAPIURL is the Resend HTTP endpoint. Declared as a package-level
// var (not const) so tests can swap it for a mock httptest server URL
// without changing the Sender constructor signature.
var resendAPIURL = "https://api.resend.com/emails"
