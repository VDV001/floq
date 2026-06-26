package onec

import (
	"context"
	"fmt"

	"github.com/daniil/floq/internal/db"
	"github.com/google/uuid"
)

// RotateSecrets re-encrypts every stored 1C auth_secret under the cipher's
// primary KEK. It reads auth_secret_enc/_nonce, decrypts it (Decrypt falls back
// to the old KEK during a rotation window), and writes back a fresh ciphertext
// sealed under the primary KEK. Returns the number re-encrypted.
//
// Invoked via `server -rotate-secrets`. The ciphertext carries no key-id, so
// every non-empty secret is re-encrypted on every run: NOT a no-op, but
// convergent and safe to repeat. A row that decrypts under neither key aborts
// the run with a wrapped error.
func RotateSecrets(ctx context.Context, q db.Querier, cipher SecretCipher) (int, error) {
	rows, err := q.Query(ctx, `
		SELECT user_id, auth_secret_enc, auth_secret_nonce FROM onec_credentials
		WHERE auth_secret_enc IS NOT NULL`)
	if err != nil {
		return 0, fmt.Errorf("rotate scan onec auth_secret: %w", err)
	}

	type pending struct {
		userID uuid.UUID
		enc    []byte
		nonce  []byte
	}
	var todo []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.userID, &p.enc, &p.nonce); err != nil {
			rows.Close()
			return 0, err
		}
		todo = append(todo, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	count := 0
	for _, p := range todo {
		plaintext, err := cipher.Decrypt(p.enc, p.nonce)
		if err != nil {
			return count, fmt.Errorf("rotate decrypt onec auth_secret (user %s): %w", p.userID, err)
		}
		ciphertext, nonce, err := cipher.Encrypt(plaintext)
		if err != nil {
			return count, fmt.Errorf("rotate encrypt onec auth_secret: %w", err)
		}
		if _, err := q.Exec(ctx, `
			UPDATE onec_credentials SET auth_secret_enc = $2, auth_secret_nonce = $3
			WHERE user_id = $1`, p.userID, ciphertext, nonce); err != nil {
			return count, fmt.Errorf("rotate update onec auth_secret: %w", err)
		}
		count++
	}
	return count, nil
}

// VerifySecretsKEK reports how many stored 1C auth_secrets decrypt under the
// given cipher and how many do not. Pass a PRIMARY-ONLY cipher to prove
// rotation is complete (bad == 0). Read-only.
func VerifySecretsKEK(ctx context.Context, q db.Querier, cipher SecretCipher) (ok, bad int, err error) {
	rows, err := q.Query(ctx, `
		SELECT auth_secret_enc, auth_secret_nonce FROM onec_credentials
		WHERE auth_secret_enc IS NOT NULL`)
	if err != nil {
		return 0, 0, fmt.Errorf("verify scan onec auth_secret: %w", err)
	}
	type pending struct{ enc, nonce []byte }
	var todo []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.enc, &p.nonce); err != nil {
			rows.Close()
			return ok, bad, err
		}
		todo = append(todo, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return ok, bad, err
	}
	for _, p := range todo {
		if _, derr := cipher.Decrypt(p.enc, p.nonce); derr == nil {
			ok++
		} else {
			bad++
		}
	}
	return ok, bad, nil
}
