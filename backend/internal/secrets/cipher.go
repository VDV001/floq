// Package secrets provides at-rest encryption for client credentials
// (IMAP/SMTP passwords, API keys, the 1C auth secret). The concrete Cipher
// implements AES-256-GCM with a per-secret random nonce, keyed by a primary
// key-encryption-key (KEK) loaded from the environment at startup, with an
// optional secondary KEK used only as a decrypt-fallback during rotation.
//
// Consumers (settings, integrations/onec) declare their own minimal
// SecretCipher interface and accept *Cipher structurally — the impl lives
// here as a leaf package, the ports live with the consumers (DIP).
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// ErrInvalidKey is returned by NewCipher when the decoded KEK is not exactly
// 32 bytes (AES-256 requires a 256-bit key).
var ErrInvalidKey = errors.New("secrets: KEK must decode to exactly 32 bytes")

// ErrDecrypt is returned when ciphertext cannot be authenticated/decrypted —
// wrong key, tampered ciphertext, or a nonce/ciphertext mismatch. It never
// reveals which, by design.
var ErrDecrypt = errors.New("secrets: decrypt failed")

// Cipher encrypts and decrypts short secret strings with AES-256-GCM.
//
// Encrypt always seals under the primary KEK. Decrypt tries primary first and,
// if a secondary (old) KEK is configured, falls back to it — this is what keeps
// reads working during a KEK rotation, when some ciphertext is still sealed
// under the previous key. The ciphertext format carries no key-id, so the
// secondary is tried structurally rather than selected.
type Cipher struct {
	primary   cipher.AEAD
	secondary cipher.AEAD // nil unless a fallback (old) KEK is configured
}

// buildAEAD decodes a base64 32-byte KEK and constructs an AES-256-GCM AEAD. A
// malformed base64 string is a distinct error; a correctly-decoded but
// wrong-length key returns ErrInvalidKey.
func buildAEAD(kekBase64 string) (cipher.AEAD, error) {
	key, err := base64.StdEncoding.DecodeString(kekBase64)
	if err != nil {
		return nil, fmt.Errorf("secrets: KEK is not valid base64: %w", err)
	}
	if len(key) != 32 {
		return nil, ErrInvalidKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secrets: build AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secrets: build GCM: %w", err)
	}
	return aead, nil
}

// NewCipher builds a single-key Cipher from a base64-encoded 32-byte KEK.
// Equivalent to NewCipherWithFallback with an empty secondary.
func NewCipher(kekBase64 string) (*Cipher, error) {
	return NewCipherWithFallback(kekBase64, "")
}

// NewCipherWithFallback builds a Cipher whose Encrypt uses primaryBase64 and
// whose Decrypt falls back to secondaryBase64 when the primary fails. An empty
// secondaryBase64 disables the fallback (behaves exactly like NewCipher). A
// present-but-malformed secondary is a hard error — it must never silently
// disable the fallback, which would lose data mid-rotation.
func NewCipherWithFallback(primaryBase64, secondaryBase64 string) (*Cipher, error) {
	primary, err := buildAEAD(primaryBase64)
	if err != nil {
		return nil, err
	}
	c := &Cipher{primary: primary}
	if secondaryBase64 != "" {
		secondary, err := buildAEAD(secondaryBase64)
		if err != nil {
			return nil, err
		}
		c.secondary = secondary
	}
	return c, nil
}

// Encrypt seals plaintext under a fresh random nonce. Empty plaintext is a
// no-op returning (nil, nil, nil) so we never store ciphertext for an unset
// secret.
func (c *Cipher) Encrypt(plaintext string) (ciphertext, nonce []byte, err error) {
	if plaintext == "" {
		return nil, nil, nil
	}
	nonce = make([]byte, c.primary.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("secrets: read nonce: %w", err)
	}
	ciphertext = c.primary.Seal(nil, nonce, []byte(plaintext), nil)
	return ciphertext, nonce, nil
}

// Decrypt opens ciphertext sealed by Encrypt. Empty (nil, nil) input is a
// no-op returning ("", nil), the inverse of Encrypt's empty-plaintext case.
// Any authentication failure collapses to ErrDecrypt.
func (c *Cipher) Decrypt(ciphertext, nonce []byte) (string, error) {
	if len(ciphertext) == 0 && len(nonce) == 0 {
		return "", nil
	}
	if len(nonce) != c.primary.NonceSize() {
		return "", ErrDecrypt
	}
	if plaintext, err := c.primary.Open(nil, nonce, ciphertext, nil); err == nil {
		return string(plaintext), nil
	}
	// Primary failed: fall back to the old KEK during a rotation window. Same
	// generic ErrDecrypt on total failure — never reveal which key matched.
	if c.secondary != nil {
		if plaintext, err := c.secondary.Open(nil, nonce, ciphertext, nil); err == nil {
			return string(plaintext), nil
		}
	}
	return "", ErrDecrypt
}
