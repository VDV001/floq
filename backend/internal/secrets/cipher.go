// Package secrets provides at-rest encryption for client credentials
// (IMAP/SMTP passwords, API keys, the 1C auth secret). The concrete Cipher
// implements AES-256-GCM with a per-secret random nonce, keyed by a single
// key-encryption-key (KEK) loaded from the environment at startup.
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
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher builds a Cipher from a base64-encoded 32-byte KEK. A malformed
// base64 string is a distinct error; a correctly-decoded but wrong-length key
// returns ErrInvalidKey.
func NewCipher(kekBase64 string) (*Cipher, error) {
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
	return &Cipher{aead: aead}, nil
}

// Encrypt seals plaintext under a fresh random nonce. Empty plaintext is a
// no-op returning (nil, nil, nil) so we never store ciphertext for an unset
// secret.
func (c *Cipher) Encrypt(plaintext string) (ciphertext, nonce []byte, err error) {
	if plaintext == "" {
		return nil, nil, nil
	}
	nonce = make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("secrets: read nonce: %w", err)
	}
	ciphertext = c.aead.Seal(nil, nonce, []byte(plaintext), nil)
	return ciphertext, nonce, nil
}

// Decrypt opens ciphertext sealed by Encrypt. Empty (nil, nil) input is a
// no-op returning ("", nil), the inverse of Encrypt's empty-plaintext case.
// Any authentication failure collapses to ErrDecrypt.
func (c *Cipher) Decrypt(ciphertext, nonce []byte) (string, error) {
	if len(ciphertext) == 0 && len(nonce) == 0 {
		return "", nil
	}
	if len(nonce) != c.aead.NonceSize() {
		return "", ErrDecrypt
	}
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecrypt
	}
	return string(plaintext), nil
}
