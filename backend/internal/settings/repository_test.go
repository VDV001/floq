package settings

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/daniil/floq/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCipher is a reversible, non-cryptographic test double for SecretCipher.
// "enc:"-prefixing makes encrypted values visibly distinct from plaintext in
// assertions while staying trivially decryptable. Empty input is a no-op,
// mirroring the real cipher's contract.
type fakeCipher struct{}

func (fakeCipher) Encrypt(plaintext string) (ciphertext, nonce []byte, err error) {
	if plaintext == "" {
		return nil, nil, nil
	}
	return []byte("enc:" + plaintext), []byte("nonce"), nil
}

func (fakeCipher) Decrypt(ciphertext, nonce []byte) (string, error) {
	if len(ciphertext) == 0 {
		return "", nil
	}
	return strings.TrimPrefix(string(ciphertext), "enc:"), nil
}

var _ SecretCipher = fakeCipher{}

// --- mock pgx helpers ---

type fakeRow struct {
	scanFn func(dest ...any) error
}

func (r *fakeRow) Scan(dest ...any) error { return r.scanFn(dest...) }

type fakeQuerier struct {
	execErr      error
	queryRowFns  []func() pgx.Row
	queryRowIdx  int

	// Captured from the most recent Exec call, for write-path assertions.
	lastExecSQL  string
	lastExecArgs []any
}

func (q *fakeQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.lastExecSQL = sql
	q.lastExecArgs = args
	return pgconn.NewCommandTag("INSERT 0 1"), q.execErr
}
func (q *fakeQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if q.queryRowIdx < len(q.queryRowFns) {
		fn := q.queryRowFns[q.queryRowIdx]
		q.queryRowIdx++
		return fn()
	}
	return &fakeRow{scanFn: func(dest ...any) error { return nil }}
}
func (q *fakeQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}

var _ db.Querier = (*fakeQuerier)(nil)

// --- Repository tests ---

func TestRepository_NewRepository(t *testing.T) {
	r := NewRepository(nil, fakeCipher{})
	require.NotNil(t, r)
}

func TestRepository_GetSettings_HappyPath(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			// user profile
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					if p, ok := dest[0].(*string); ok { *p = "John Doe" }
					if p, ok := dest[1].(*string); ok { *p = "john@example.com" }
					return nil
				}}
			},
			// user_settings — projection after 047 (no plaintext secret cols):
			// telegram_bot_active(0), ai_provider(7), ai_model(8); the
			// telegram_bot_token comes from its ciphertext at index 21/22.
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					if p, ok := dest[0].(*bool); ok { *p = true }            // telegram_bot_active
					if p, ok := dest[7].(*string); ok { *p = "openai" }      // ai_provider
					if p, ok := dest[8].(*string); ok { *p = "gpt-4o" }      // ai_model
					if len(dest) > 22 {
						if p, ok := dest[21].(*[]byte); ok { *p = []byte("enc:bot-token-123") }
						if p, ok := dest[22].(*[]byte); ok { *p = []byte("nonce") }
					}
					return nil
				}}
			},
		},
	}

	r := NewRepositoryFromQuerier(q, fakeCipher{})
	s, err := r.GetSettings(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "John Doe", s.FullName)
	assert.Equal(t, "john@example.com", s.Email)
	assert.Equal(t, "bot-token-123", s.TelegramBotToken)
	assert.Equal(t, "openai", s.AIProvider)
}

func TestRepository_GetSettings_NoSettingsRow(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					if p, ok := dest[0].(*string); ok { *p = "Jane" }
					if p, ok := dest[1].(*string); ok { *p = "jane@x.com" }
					return nil
				}}
			},
			// user_settings row not found
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					return pgx.ErrNoRows
				}}
			},
		},
	}

	r := NewRepositoryFromQuerier(q, fakeCipher{})
	s, err := r.GetSettings(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "Jane", s.FullName)
	// defaults should be applied
	assert.Equal(t, "993", s.IMAPPort)
	assert.Equal(t, "ollama", s.AIProvider)
}

func TestRepository_GetSettings_UserNotFound(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					return pgx.ErrNoRows
				}}
			},
		},
	}

	r := NewRepositoryFromQuerier(q, fakeCipher{})
	_, err := r.GetSettings(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "load user profile")
}

func TestRepository_UpdateSettings_HappyPath(t *testing.T) {
	q := &fakeQuerier{}
	r := NewRepositoryFromQuerier(q, fakeCipher{})

	err := r.UpdateSettings(context.Background(), uuid.New(), map[string]any{
		"auto_qualify": true,
		"ai_provider":  "openai",
	})
	assert.NoError(t, err)
}

func TestRepository_UpdateSettings_ExecError(t *testing.T) {
	q := &fakeQuerier{execErr: fmt.Errorf("db error")}
	r := NewRepositoryFromQuerier(q, fakeCipher{})

	err := r.UpdateSettings(context.Background(), uuid.New(), map[string]any{"auto_qualify": true})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "save settings")
}

func TestRepository_UpdateSettings_Empty(t *testing.T) {
	q := &fakeQuerier{}
	r := NewRepositoryFromQuerier(q, fakeCipher{})

	err := r.UpdateSettings(context.Background(), uuid.New(), map[string]any{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no fields")
}

func TestRepository_GetStoredIMAPPassword(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					// SELECT imap_password_enc, imap_password_nonce (047 dropped plaintext).
					if p, ok := dest[0].(*[]byte); ok { *p = []byte("enc:secret-password") }
					if p, ok := dest[1].(*[]byte); ok { *p = []byte("nonce") }
					return nil
				}}
			},
		},
	}
	r := NewRepositoryFromQuerier(q, fakeCipher{})

	pwd, err := r.GetStoredIMAPPassword(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "secret-password", pwd)
}

func TestRepository_GetStoredIMAPPassword_NotFound(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					return pgx.ErrNoRows
				}}
			},
		},
	}
	r := NewRepositoryFromQuerier(q, fakeCipher{})

	_, err := r.GetStoredIMAPPassword(context.Background(), uuid.New())
	assert.Error(t, err)
}

// --- Store tests ---

func TestStore_NewStore(t *testing.T) {
	s := NewStore(nil, fakeCipher{})
	require.NotNil(t, s)
}

func TestStore_GetConfig_HappyPath(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					// GetConfig projection after 047: smtp_host(0), ai_provider(3),
					// then enc/nonce pairs. resend_api_key comes from its
					// ciphertext (resend_api_key_enc at index 8).
					if p, ok := dest[0].(*string); ok { *p = "smtp.test.com" } // smtp_host
					if p, ok := dest[3].(*string); ok { *p = "openai" }        // ai_provider
					if p, ok := dest[8].(*[]byte); ok { *p = []byte("enc:re_key_123") }
					if p, ok := dest[9].(*[]byte); ok { *p = []byte("nonce") }
					return nil
				}}
			},
		},
	}
	s := NewStoreFromQuerier(q, fakeCipher{})

	cfg, err := s.GetConfig(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "re_key_123", cfg.ResendAPIKey)
	assert.Equal(t, "smtp.test.com", cfg.SMTPHost)
}

func TestStore_GetConfig_NoRow(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					return pgx.ErrNoRows
				}}
			},
		},
	}
	s := NewStoreFromQuerier(q, fakeCipher{})

	cfg, err := s.GetConfig(context.Background(), uuid.New())
	require.NoError(t, err)
	// Returns empty config, not error
	assert.Equal(t, "", cfg.ResendAPIKey)
}

// --- at-rest encryption boundary ---

func TestRepository_UpdateSettings_EncryptsSecretColumn(t *testing.T) {
	q := &fakeQuerier{}
	r := NewRepositoryFromQuerier(q, fakeCipher{})

	err := r.UpdateSettings(context.Background(), uuid.New(), map[string]any{
		"imap_password": "secret-pw",
	})
	require.NoError(t, err)

	// Writes ONLY the encrypted byte columns — the legacy plaintext column was
	// dropped in migration 047, so it must not appear in the write at all.
	assert.Contains(t, q.lastExecSQL, "imap_password_enc")
	assert.Contains(t, q.lastExecSQL, "imap_password_nonce")
	assert.NotContains(t, q.lastExecSQL, "imap_password = ''")
	assert.NotContains(t, q.lastExecSQL, "imap_password,")

	foundCT := false
	for _, a := range q.lastExecArgs {
		if b, ok := a.([]byte); ok && string(b) == "enc:secret-pw" {
			foundCT = true
		}
	}
	assert.True(t, foundCT, "ciphertext arg should be present")
}

func TestRepository_UpdateSettings_NonSecretWrittenPlaintext(t *testing.T) {
	q := &fakeQuerier{}
	r := NewRepositoryFromQuerier(q, fakeCipher{})

	err := r.UpdateSettings(context.Background(), uuid.New(), map[string]any{
		"ai_provider": "openai",
	})
	require.NoError(t, err)

	assert.Contains(t, q.lastExecSQL, "ai_provider")
	assert.NotContains(t, q.lastExecSQL, "ai_provider_enc")

	found := false
	for _, a := range q.lastExecArgs {
		if s, ok := a.(string); ok && s == "openai" {
			found = true
		}
	}
	assert.True(t, found, "plaintext value should pass through unchanged")
}

func TestRepository_GetSettings_DecryptsEncColumns(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					if p, ok := dest[0].(*string); ok { *p = "U" }
					if p, ok := dest[1].(*string); ok { *p = "u@x" }
					return nil
				}}
			},
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					// Encrypted imap_password at indices 23/24 (plaintext column
					// dropped in 047, so it is no longer in the projection).
					if len(dest) > 24 {
						if p, ok := dest[23].(*[]byte); ok { *p = []byte("enc:fresh-pw") }
						if p, ok := dest[24].(*[]byte); ok { *p = []byte("nonce") }
					}
					return nil
				}}
			},
		},
	}
	r := NewRepositoryFromQuerier(q, fakeCipher{})
	s, err := r.GetSettings(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "fresh-pw", s.IMAPPassword)
}

func TestRepository_GetStoredIMAPPassword_DecryptsEnc(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					// SELECT imap_password_enc, imap_password_nonce.
					if len(dest) > 1 {
						if p, ok := dest[0].(*[]byte); ok { *p = []byte("enc:real-pw") }
						if p, ok := dest[1].(*[]byte); ok { *p = []byte("nonce") }
					}
					return nil
				}}
			},
		},
	}
	r := NewRepositoryFromQuerier(q, fakeCipher{})
	pwd, err := r.GetStoredIMAPPassword(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "real-pw", pwd)
}

func TestStore_GetConfig_PropagatesScanError(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					return fmt.Errorf("db boom")
				}}
			},
		},
	}
	s := NewStoreFromQuerier(q, fakeCipher{})

	_, err := s.GetConfig(context.Background(), uuid.New())
	assert.Error(t, err, "a real DB/decode error must not be masked as an empty config")
}

func TestStore_GetConfig_DecryptsSecrets(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					// GetConfig projection (plaintext columns dropped in 047):
					// imap_password_enc/_nonce land at indices 14/15.
					if len(dest) > 15 {
						if p, ok := dest[14].(*[]byte); ok { *p = []byte("enc:imap-real") }
						if p, ok := dest[15].(*[]byte); ok { *p = []byte("nonce") }
					}
					return nil
				}}
			},
		},
	}
	s := NewStoreFromQuerier(q, fakeCipher{})
	cfg, err := s.GetConfig(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "imap-real", cfg.IMAPPassword)
}
