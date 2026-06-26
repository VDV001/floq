//go:build integration

package testutil

import (
	"encoding/base64"
	"testing"

	"github.com/daniil/floq/internal/secrets"
	"github.com/stretchr/testify/require"
)

// NewSecretCipher builds a real AES-256-GCM cipher for integration round-trips,
// so the at-rest encryption path is exercised against actual crypto rather than
// a stub. Shared by the settings and onec integration tests.
func NewSecretCipher(t *testing.T) *secrets.Cipher {
	t.Helper()
	key := base64.StdEncoding.EncodeToString([]byte("floq-integration-test-kek-32byte"))
	c, err := secrets.NewCipher(key)
	require.NoError(t, err)
	return c
}
