package settings

import "errors"

// Sentinel errors classifying Resend-tester failures. Same pattern as
// SMTP: helpers.go wraps the underlying I/O error with %w against one
// of these; settings/handler.go translates to a user-facing Russian
// string via resendErrorToUserMessage.
var (
	// ErrResendRequest — request build/transport-level failure
	// (http.NewRequestWithContext error or client.Do error).
	ErrResendRequest = errors.New("resend request")

	// ErrResendAuth — Resend API returned a non-200 status, treated as
	// "bad API key" for the purposes of the connection-test endpoint.
	ErrResendAuth = errors.New("resend auth")
)
