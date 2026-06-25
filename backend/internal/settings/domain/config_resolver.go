package domain

// ResolveConfig returns the DB value if non-empty, otherwise the fallback.
func ResolveConfig(dbValue, fallback string) string {
	if dbValue != "" {
		return dbValue
	}
	return fallback
}

// IsEmailConfigured reports whether outbound email can be sent with the given
// resolved credentials: a Resend API key, or a full SMTP triple (host, user,
// password). This is the settings-owned rule for "email is set up"; callers
// resolve DB-then-env first (via ResolveConfig) and pass the effective values.
func IsEmailConfigured(resendKey, smtpHost, smtpUser, smtpPassword string) bool {
	return resendKey != "" || (smtpHost != "" && smtpUser != "" && smtpPassword != "")
}
