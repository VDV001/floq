//go:build integration

package onec_test

import (
	"encoding/base64"
	"testing"

	"github.com/daniil/floq/internal/secrets"
	"github.com/stretchr/testify/require"
)

// testCipher builds a real AES-256-GCM cipher for integration round-trips, so
// the at-rest encryption path is exercised against actual crypto and a real
// database rather than a stub.
func testCipher(t *testing.T) *secrets.Cipher {
	t.Helper()
	key := base64.StdEncoding.EncodeToString([]byte("onec-integration-test-kek-32byte"))
	c, err := secrets.NewCipher(key)
	require.NoError(t, err)
	return c
}
