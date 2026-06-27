package webhooks

// SecretCipher encrypts and decrypts a webhook endpoint's signing secret at the
// storage boundary. Declared here, in the consumer, per DIP; the AES-256-GCM
// implementation lives in internal/secrets and is injected from the composition
// root. The two-method shape is intentionally duplicated with the settings/onec
// packages — idiomatic "accept interfaces" in Go, not a DRY violation. The
// secret must be reversibly encrypted (not hashed): the delivery worker needs
// the plaintext to compute the HMAC signature.
type SecretCipher interface {
	Encrypt(plaintext string) (ciphertext, nonce []byte, err error)
	Decrypt(ciphertext, nonce []byte) (string, error)
}
