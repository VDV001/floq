package settings

import "errors"

// Sentinel errors classifying SMTP-tester failures. The composition root
// (cmd/server/helpers.go) wraps the underlying I/O error with %w against
// one of these so the settings handler can translate to a user-facing
// Russian string via smtpErrorToUserMessage. The architectural rule is:
// helpers.go produces typed errors only — no UI strings; settings/handler.go
// owns the user-facing copy.
var (
	// ErrSMTPProxyDial — TCP dial through the configured proxy failed.
	ErrSMTPProxyDial = errors.New("smtp proxy dial")

	// ErrSMTPDial — direct TCP/TLS dial to the SMTP host failed.
	ErrSMTPDial = errors.New("smtp dial")

	// ErrSMTPClient — smtp.NewClient on the established connection failed
	// (server greeting unparsable or sent unexpected response).
	ErrSMTPClient = errors.New("smtp client")

	// ErrSMTPStartTLS — STARTTLS handshake failed on a plain-TCP SMTP
	// connection (port 587 / 25 path).
	ErrSMTPStartTLS = errors.New("smtp starttls")

	// ErrSMTPAuth — server rejected the supplied credentials.
	ErrSMTPAuth = errors.New("smtp auth")
)
