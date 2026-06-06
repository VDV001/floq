package settings

// decryptOrFallback returns the decrypted enc value when ciphertext is
// present, otherwise the legacy plaintext column. The plaintext fallback
// exists only for the transition window: rows written before migration 037
// (or not yet backfilled) still carry their secret in the TEXT column. Once
// the backfill is verified and migration 038 drops the plaintext columns,
// this fallback becomes dead and is removed.
func decryptOrFallback(cipher SecretCipher, ciphertext, nonce []byte, plaintext string) (string, error) {
	if len(ciphertext) == 0 {
		return plaintext, nil
	}
	return cipher.Decrypt(ciphertext, nonce)
}
