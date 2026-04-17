package settings

import (
	"context"
	"fmt"
	"testing"

	"github.com/daniil/floq/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock pgx helpers ---

type fakeRow struct {
	scanFn func(dest ...any) error
}

func (r *fakeRow) Scan(dest ...any) error { return r.scanFn(dest...) }

type fakeQuerier struct {
	execErr      error
	queryRowFns  []func() pgx.Row
	queryRowIdx  int
}

func (q *fakeQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
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
	r := NewRepository(nil)
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
			// user_settings
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					// Set some fields via scan
					if p, ok := dest[0].(*string); ok { *p = "bot-token-123" }    // telegram_bot_token
					if p, ok := dest[1].(*bool); ok { *p = true }                  // telegram_bot_active
					if p, ok := dest[11].(*string); ok { *p = "openai" }            // ai_provider
					if p, ok := dest[12].(*string); ok { *p = "gpt-4o" }            // ai_model
					return nil
				}}
			},
		},
	}

	r := NewRepositoryFromQuerier(q)
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

	r := NewRepositoryFromQuerier(q)
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

	r := NewRepositoryFromQuerier(q)
	_, err := r.GetSettings(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "load user profile")
}

func TestRepository_UpdateSettings_HappyPath(t *testing.T) {
	q := &fakeQuerier{}
	r := NewRepositoryFromQuerier(q)

	err := r.UpdateSettings(context.Background(), uuid.New(), map[string]any{
		"auto_qualify": true,
		"ai_provider":  "openai",
	})
	assert.NoError(t, err)
}

func TestRepository_UpdateSettings_ExecError(t *testing.T) {
	q := &fakeQuerier{execErr: fmt.Errorf("db error")}
	r := NewRepositoryFromQuerier(q)

	err := r.UpdateSettings(context.Background(), uuid.New(), map[string]any{"auto_qualify": true})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "save settings")
}

func TestRepository_UpdateSettings_Empty(t *testing.T) {
	q := &fakeQuerier{}
	r := NewRepositoryFromQuerier(q)

	err := r.UpdateSettings(context.Background(), uuid.New(), map[string]any{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no fields")
}

func TestRepository_GetStoredIMAPPassword(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					if p, ok := dest[0].(*string); ok { *p = "secret-password" }
					return nil
				}}
			},
		},
	}
	r := NewRepositoryFromQuerier(q)

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
	r := NewRepositoryFromQuerier(q)

	_, err := r.GetStoredIMAPPassword(context.Background(), uuid.New())
	assert.Error(t, err)
}

// --- Store tests ---

func TestStore_NewStore(t *testing.T) {
	s := NewStore(nil)
	require.NotNil(t, s)
}

func TestStore_GetConfig_HappyPath(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					if p, ok := dest[0].(*string); ok { *p = "re_key_123" }   // resend_api_key
					if p, ok := dest[1].(*string); ok { *p = "smtp.test.com" } // smtp_host
					if p, ok := dest[5].(*string); ok { *p = "openai" }        // ai_provider
					return nil
				}}
			},
		},
	}
	s := NewStoreFromQuerier(q)

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
	s := NewStoreFromQuerier(q)

	cfg, err := s.GetConfig(context.Background(), uuid.New())
	require.NoError(t, err)
	// Returns empty config, not error
	assert.Equal(t, "", cfg.ResendAPIKey)
}
