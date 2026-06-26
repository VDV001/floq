package secrets

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// key32 returns a base64-encoded 32-byte key whose every byte is b.
func key32(b byte) string {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = b
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func TestNewCipher_RejectsShortKey(t *testing.T) {
	short := base64.StdEncoding.EncodeToString(make([]byte, 16))
	_, err := NewCipher(short)
	assert.ErrorIs(t, err, ErrInvalidKey)
}

func TestNewCipher_RejectsInvalidBase64(t *testing.T) {
	_, err := NewCipher("!!!! not base64 !!!!")
	assert.Error(t, err)
}

func TestNewCipher_AcceptsValid32ByteKey(t *testing.T) {
	c, err := NewCipher(key32(0x01))
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func TestCipher_RoundTrip(t *testing.T) {
	c, err := NewCipher(key32(0x01))
	require.NoError(t, err)

	ct, nonce, err := c.Encrypt("imap-password-123")
	require.NoError(t, err)
	require.NotEmpty(t, ct)
	require.NotEmpty(t, nonce)

	got, err := c.Decrypt(ct, nonce)
	require.NoError(t, err)
	assert.Equal(t, "imap-password-123", got)
}

func TestCipher_CiphertextDoesNotContainPlaintext(t *testing.T) {
	c, err := NewCipher(key32(0x01))
	require.NoError(t, err)

	ct, _, err := c.Encrypt("supersecret")
	require.NoError(t, err)
	assert.NotContains(t, string(ct), "supersecret")
}

func TestCipher_WrongKeyFailsToDecrypt(t *testing.T) {
	enc, err := NewCipher(key32(0x01))
	require.NoError(t, err)
	dec, err := NewCipher(key32(0x02))
	require.NoError(t, err)

	ct, nonce, err := enc.Encrypt("secret")
	require.NoError(t, err)

	_, err = dec.Decrypt(ct, nonce)
	assert.ErrorIs(t, err, ErrDecrypt)
}

func TestCipher_TamperedCiphertextFails(t *testing.T) {
	c, err := NewCipher(key32(0x01))
	require.NoError(t, err)

	ct, nonce, err := c.Encrypt("secret")
	require.NoError(t, err)
	ct[0] ^= 0xFF // flip a bit

	_, err = c.Decrypt(ct, nonce)
	assert.ErrorIs(t, err, ErrDecrypt)
}

func TestCipher_EmptyPlaintextIsNoOp(t *testing.T) {
	c, err := NewCipher(key32(0x01))
	require.NoError(t, err)

	ct, nonce, err := c.Encrypt("")
	require.NoError(t, err)
	assert.Nil(t, ct)
	assert.Nil(t, nonce)

	got, err := c.Decrypt(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestNewCipherWithFallback_EmptySecondaryEqualsSingleKey(t *testing.T) {
	// An empty secondary KEK must behave exactly like the single-key NewCipher:
	// encrypt under primary, decrypt under primary, no fallback.
	c, err := NewCipherWithFallback(key32(0x01), "")
	require.NoError(t, err)

	ct, nonce, err := c.Encrypt("secret")
	require.NoError(t, err)
	got, err := c.Decrypt(ct, nonce)
	require.NoError(t, err)
	assert.Equal(t, "secret", got)
}

func TestNewCipherWithFallback_RejectsInvalidSecondary(t *testing.T) {
	// A present-but-malformed secondary must fail fast, never silently disable
	// the fallback (which would lose data during a rotation window).
	_, err := NewCipherWithFallback(key32(0x01), base64.StdEncoding.EncodeToString(make([]byte, 16)))
	assert.ErrorIs(t, err, ErrInvalidKey)

	_, err = NewCipherWithFallback(key32(0x01), "!!!! not base64 !!!!")
	assert.Error(t, err)
}

func TestCipherWithFallback_DecryptsSecondaryKeyCiphertext(t *testing.T) {
	// Ciphertext sealed under the OLD key must decrypt via the secondary
	// fallback — this is what keeps reads working during a KEK rotation.
	old, err := NewCipher(key32(0xAA))
	require.NoError(t, err)
	ct, nonce, err := old.Encrypt("rotated-secret")
	require.NoError(t, err)

	// primary = new key, secondary = old key.
	c, err := NewCipherWithFallback(key32(0xBB), key32(0xAA))
	require.NoError(t, err)

	got, err := c.Decrypt(ct, nonce)
	require.NoError(t, err)
	assert.Equal(t, "rotated-secret", got)
}

func TestCipherWithFallback_DecryptsPrimaryKeyCiphertext(t *testing.T) {
	// Primary must still be tried first and succeed for new-key ciphertext.
	c, err := NewCipherWithFallback(key32(0xBB), key32(0xAA))
	require.NoError(t, err)

	ct, nonce, err := c.Encrypt("new-secret")
	require.NoError(t, err)

	got, err := c.Decrypt(ct, nonce)
	require.NoError(t, err)
	assert.Equal(t, "new-secret", got)
}

func TestCipherWithFallback_EncryptAlwaysUsesPrimary(t *testing.T) {
	// Encrypt must seal under primary only: a primary-only cipher decrypts it,
	// an old-key-only cipher must NOT. This is the property that lets
	// -verify-secrets-kek (primary-only) prove rotation completeness.
	c, err := NewCipherWithFallback(key32(0xBB), key32(0xAA))
	require.NoError(t, err)
	ct, nonce, err := c.Encrypt("new-secret")
	require.NoError(t, err)

	primaryOnly, err := NewCipher(key32(0xBB))
	require.NoError(t, err)
	got, err := primaryOnly.Decrypt(ct, nonce)
	require.NoError(t, err)
	assert.Equal(t, "new-secret", got)

	oldOnly, err := NewCipher(key32(0xAA))
	require.NoError(t, err)
	_, err = oldOnly.Decrypt(ct, nonce)
	assert.ErrorIs(t, err, ErrDecrypt)
}

func TestCipherWithFallback_UnknownKeyStillFails(t *testing.T) {
	// Ciphertext under a third key decrypts under neither primary nor secondary.
	stray, err := NewCipher(key32(0xCC))
	require.NoError(t, err)
	ct, nonce, err := stray.Encrypt("secret")
	require.NoError(t, err)

	c, err := NewCipherWithFallback(key32(0xBB), key32(0xAA))
	require.NoError(t, err)
	_, err = c.Decrypt(ct, nonce)
	assert.ErrorIs(t, err, ErrDecrypt)
}

func TestCipherWithFallback_EmptyPlaintextIsNoOp(t *testing.T) {
	c, err := NewCipherWithFallback(key32(0xBB), key32(0xAA))
	require.NoError(t, err)

	ct, nonce, err := c.Encrypt("")
	require.NoError(t, err)
	assert.Nil(t, ct)
	assert.Nil(t, nonce)

	got, err := c.Decrypt(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestCipher_NonceIsRandomPerCall(t *testing.T) {
	c, err := NewCipher(key32(0x01))
	require.NoError(t, err)

	ct1, nonce1, err := c.Encrypt("same-plaintext")
	require.NoError(t, err)
	ct2, nonce2, err := c.Encrypt("same-plaintext")
	require.NoError(t, err)

	// Same input, different nonce → different ciphertext (no deterministic leak).
	assert.NotEqual(t, nonce1, nonce2)
	assert.NotEqual(t, ct1, ct2)
}
