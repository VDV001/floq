package settings

import (
	"context"
	"fmt"

	"github.com/daniil/floq/internal/db"
	"github.com/google/uuid"
)

// BackfillSecrets encrypts every legacy plaintext secret that has no
// ciphertext yet (enc IS NULL) into its <col>_enc/<col>_nonce columns. It is
// idempotent — empty or already-encrypted secrets are skipped — and returns
// the number of secrets encrypted.
//
// It runs automatically at startup BEFORE migrations (see cmd/server
// autoBackfillSecrets), so any straggler plaintext secret is encrypted before
// migration 047's drop-guard runs — making that guard a never-fire safety net
// on a normally-booted server. Requires the plaintext columns to still exist
// (pre-047); the caller gates on that.
func BackfillSecrets(ctx context.Context, q db.Querier, cipher SecretCipher) (int, error) {
	count := 0
	for col := range secretColumns {
		// col is one of a fixed internal set (secretColumns), never user
		// input, so interpolating it into the SQL is safe.
		rows, err := q.Query(ctx, fmt.Sprintf(
			`SELECT user_id, %s FROM user_settings WHERE %s <> '' AND %s_enc IS NULL`,
			col, col, col))
		if err != nil {
			return count, fmt.Errorf("backfill scan %s: %w", col, err)
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
				return count, err
			}
			todo = append(todo, p)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return count, err
		}

		for _, p := range todo {
			ciphertext, nonce, err := cipher.Encrypt(p.plaintext)
			if err != nil {
				return count, fmt.Errorf("backfill encrypt %s: %w", col, err)
			}
			if _, err := q.Exec(ctx, fmt.Sprintf(
				`UPDATE user_settings SET %s_enc = $2, %s_nonce = $3 WHERE user_id = $1`, col, col),
				p.userID, ciphertext, nonce); err != nil {
				return count, fmt.Errorf("backfill update %s: %w", col, err)
			}
			count++
		}
	}
	return count, nil
}
