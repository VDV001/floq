package onec

// SecretCipher encrypts and decrypts the 1C auth secret at the storage
// boundary. Declared here, in the consumer, per DIP; the AES-256-GCM
// implementation lives in internal/secrets and is injected from the
// composition root. The two-method shape is intentionally duplicated with the
// settings package — idiomatic "accept interfaces" in Go, not a DRY
// violation.
type SecretCipher interface {
	Encrypt(plaintext string) (ciphertext, nonce []byte, err error)
	Decrypt(ciphertext, nonce []byte) (string, error)
}

// decryptOrFallback returns the decrypted ciphertext when present, otherwise
// the legacy plaintext column. The plaintext fallback covers rows written
// before migration 037 (or not yet backfilled) and is removed once migration
// 038 drops the plaintext column.
func decryptOrFallback(cipher SecretCipher, ciphertext, nonce []byte, plaintext string) (string, error) {
	if len(ciphertext) == 0 {
		return plaintext, nil
	}
	return cipher.Decrypt(ciphertext, nonce)
}
