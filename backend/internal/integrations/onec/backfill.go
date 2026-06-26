package onec

import (
	"context"
	"fmt"

	"github.com/daniil/floq/internal/db"
	"github.com/google/uuid"
)

// BackfillSecrets encrypts every legacy plaintext 1C auth secret that has no
// ciphertext yet (auth_secret_enc IS NULL) into auth_secret_enc/_nonce. It is
// idempotent — empty or already-encrypted secrets are skipped — and returns
// the number encrypted.
//
// Run once after migration 037 (server -backfill-secrets) and before
// migration 038 drops the plaintext column. The plaintext column is left
// intact so the read path can still fall back until 038.
func BackfillSecrets(ctx context.Context, q db.Querier, cipher SecretCipher) (int, error) {
	rows, err := q.Query(ctx, `
		SELECT user_id, auth_secret FROM onec_credentials
		WHERE auth_secret <> '' AND auth_secret_enc IS NULL`)
	if err != nil {
		return 0, fmt.Errorf("backfill scan onec auth_secret: %w", err)
	}

	type pending struct {
		userID    uuid.UUID
		plaintext string
	}
	var todo []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.userID, &p.plaintext); err != nil {
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
		ciphertext, nonce, err := cipher.Encrypt(p.plaintext)
		if err != nil {
			return count, fmt.Errorf("backfill encrypt onec auth_secret: %w", err)
		}
		if _, err := q.Exec(ctx, `
			UPDATE onec_credentials SET auth_secret_enc = $2, auth_secret_nonce = $3
			WHERE user_id = $1`, p.userID, ciphertext, nonce); err != nil {
			return count, fmt.Errorf("backfill update onec auth_secret: %w", err)
		}
		count++
	}
	return count, nil
}
