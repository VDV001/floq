// Package secrets provides at-rest encryption for client credentials
// (IMAP/SMTP passwords, API keys, the 1C auth secret). The concrete Cipher
// implements AES-256-GCM with a per-secret random nonce, keyed by a single
// key-encryption-key (KEK) loaded from the environment at startup.
//
// Consumers (settings, integrations/onec) declare their own minimal
// SecretCipher interface and accept *Cipher structurally — the impl lives
// here as a leaf package, the ports live with the consumers (DIP).
package secrets

import "errors"

// ErrInvalidKey is returned by NewCipher when the decoded KEK is not exactly
// 32 bytes (AES-256 requires a 256-bit key).
var ErrInvalidKey = errors.New("secrets: KEK must decode to exactly 32 bytes")

// ErrDecrypt is returned when ciphertext cannot be authenticated/decrypted —
// wrong key, tampered ciphertext, or a nonce/ciphertext mismatch. It never
// reveals which, by design.
var ErrDecrypt = errors.New("secrets: decrypt failed")

// Cipher encrypts and decrypts short secret strings with AES-256-GCM.
type Cipher struct{}

// NewCipher builds a Cipher from a base64-encoded 32-byte KEK.
func NewCipher(kekBase64 string) (*Cipher, error) {
	return nil, errors.New("secrets: not implemented")
}

// Encrypt seals plaintext under a fresh random nonce. Empty plaintext is a
// no-op returning (nil, nil, nil) so we never store ciphertext for an unset
// secret.
func (c *Cipher) Encrypt(plaintext string) (ciphertext, nonce []byte, err error) {
	return nil, nil, errors.New("secrets: not implemented")
}

// Decrypt opens ciphertext sealed by Encrypt. Empty (nil, nil) input is a
// no-op returning ("", nil), the inverse of Encrypt's empty-plaintext case.
func (c *Cipher) Decrypt(ciphertext, nonce []byte) (string, error) {
	return "", errors.New("secrets: not implemented")
}
