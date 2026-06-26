package settings

import (
	"context"
	"fmt"

	"github.com/daniil/floq/internal/db"
	"github.com/google/uuid"
)

// RotateSecrets re-encrypts every stored user_settings secret under the
// cipher's primary KEK. It reads each <col>_enc/<col>_nonce, decrypts it
// (the cipher's Decrypt falls back to the old KEK during a rotation window),
// and writes back a fresh ciphertext sealed under the primary KEK. Returns the
// number of secrets re-encrypted.
//
// Invoked via `server -rotate-secrets`. Because the ciphertext carries no
// key-id, the rotation cannot tell which key a row already uses, so every
// non-empty secret is re-encrypted on every run: the operation is NOT a no-op
// but is convergent and safe to repeat. A row that decrypts under neither the
// primary nor the fallback KEK aborts the run with a wrapped error, surfacing
// the bad key configuration to the operator before any data is touched further.
func RotateSecrets(ctx context.Context, q db.Querier, cipher SecretCipher) (int, error) {
	count := 0
	for col := range secretColumns {
		// col is one of a fixed internal set (secretColumns), never user
		// input, so interpolating it into the SQL is safe.
		rows, err := q.Query(ctx, fmt.Sprintf(
			`SELECT user_id, %s_enc, %s_nonce FROM user_settings WHERE %s_enc IS NOT NULL`,
			col, col, col))
		if err != nil {
			return count, fmt.Errorf("rotate scan %s: %w", col, err)
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
				return count, err
			}
			todo = append(todo, p)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return count, err
		}

		for _, p := range todo {
			plaintext, err := cipher.Decrypt(p.enc, p.nonce)
			if err != nil {
				return count, fmt.Errorf("rotate decrypt %s (user %s): %w", col, p.userID, err)
			}
			ciphertext, nonce, err := cipher.Encrypt(plaintext)
			if err != nil {
				return count, fmt.Errorf("rotate encrypt %s: %w", col, err)
			}
			if _, err := q.Exec(ctx, fmt.Sprintf(
				`UPDATE user_settings SET %s_enc = $2, %s_nonce = $3 WHERE user_id = $1`, col, col),
				p.userID, ciphertext, nonce); err != nil {
				return count, fmt.Errorf("rotate update %s: %w", col, err)
			}
			count++
		}
	}
	return count, nil
}

// VerifySecretsKEK reports how many stored user_settings secrets decrypt under
// the given cipher and how many do not. Pass a PRIMARY-ONLY cipher (no
// fallback) to prove rotation is complete: bad == 0 means every secret is
// sealed under the current primary KEK, so the old KEK can be retired. It is
// read-only and writes nothing.
func VerifySecretsKEK(ctx context.Context, q db.Querier, cipher SecretCipher) (ok, bad int, err error) {
	for col := range secretColumns {
		rows, err := q.Query(ctx, fmt.Sprintf(
			`SELECT %s_enc, %s_nonce FROM user_settings WHERE %s_enc IS NOT NULL`, col, col, col))
		if err != nil {
			return ok, bad, fmt.Errorf("verify scan %s: %w", col, err)
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
	}
	return ok, bad, nil
}
