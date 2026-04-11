package domain

// ResolveConfig returns the DB value if non-empty, otherwise the fallback.
func ResolveConfig(dbValue, fallback string) string {
	if dbValue != "" {
		return dbValue
	}
	return fallback
}
