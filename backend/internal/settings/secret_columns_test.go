package settings

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSecretColumns_PinsEncryptedSet pins the exact set of encrypted columns.
// BackfillSecrets, RotateSecrets and VerifySecretsKEK all iterate secretColumns,
// so a silent drift (a 6th encrypted column added without updating this set, or
// one dropped) would remove that column from every secret path at once — the
// migration-047 "1-of-N predicate" failure mode. Changing the set must be a
// deliberate edit that also updates this test.
func TestSecretColumns_PinsEncryptedSet(t *testing.T) {
	want := []string{
		"ai_api_key",
		"imap_password",
		"resend_api_key",
		"smtp_password",
		"telegram_bot_token",
	}
	var got []string
	for col := range secretColumns {
		got = append(got, col)
	}
	sort.Strings(got)
	assert.Equal(t, want, got)
}
