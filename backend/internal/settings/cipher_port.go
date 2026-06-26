package settings

// SecretCipher encrypts and decrypts secret values at the storage boundary.
// It is declared here, in the consumer, per DIP: the concrete AES-256-GCM
// implementation lives in internal/secrets and is injected from the
// composition root. Encrypt returns (nil, nil, nil) for empty input and
// Decrypt is its inverse, so callers can treat an unset secret uniformly.
type SecretCipher interface {
	Encrypt(plaintext string) (ciphertext, nonce []byte, err error)
	Decrypt(ciphertext, nonce []byte) (string, error)
}

// secretColumns are the user_settings columns persisted encrypted: their
// plaintext lives in <col>_enc/<col>_nonce byte columns, never the legacy
// TEXT column (kept only for read-fallback until migration 038 drops it).
// Single source of truth for both the read (decrypt) and write (encrypt)
// paths.
var secretColumns = map[string]bool{
	"telegram_bot_token": true,
	"imap_password":      true,
	"resend_api_key":     true,
	"smtp_password":      true,
	"ai_api_key":         true,
}
