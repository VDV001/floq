package settings

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/daniil/floq/internal/settings/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRepository_GetSettings_AIStyleCheckEnabledDefaultsTrue asserts that
// when no user_settings row exists yet, GetSettings returns
// AIStyleCheckEnabled = true. The default is opt-out (style-check on for
// all new users) because cold-outreach quality is the primary motivation
// for adding the feature; users who care about latency can disable it.
func TestRepository_GetSettings_AIStyleCheckEnabledDefaultsTrue(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			// user profile
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					if p, ok := dest[0].(*string); ok {
						*p = "Jane"
					}
					if p, ok := dest[1].(*string); ok {
						*p = "jane@x.com"
					}
					return nil
				}}
			},
			// user_settings row not found
			func() pgx.Row {
				return &fakeRow{scanFn: func(_ ...any) error { return pgx.ErrNoRows }}
			},
		},
	}

	r := NewRepositoryFromQuerier(q, fakeCipher{})
	s, err := r.GetSettings(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.True(t, s.AIStyleCheckEnabled,
		"AIStyleCheckEnabled must default to true when no user_settings row exists")
}

// TestRepository_GetSettings_AIStyleCheckEnabledFromDB asserts that an
// explicit false from the DB row overrides the default and reaches the
// Settings entity.
func TestRepository_GetSettings_AIStyleCheckEnabledFromDB(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					if p, ok := dest[0].(*string); ok {
						*p = "Jane"
					}
					if p, ok := dest[1].(*string); ok {
						*p = "jane@x.com"
					}
					return nil
				}}
			},
			// user_settings row: ai_style_check_enabled sits at scan index 19
			// after migration 047 dropped the 5 plaintext secret columns from
			// the GetSettings projection.
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					if p, ok := dest[19].(*bool); ok {
						*p = false
					}
					return nil
				}}
			},
		},
	}

	r := NewRepositoryFromQuerier(q, fakeCipher{})
	s, err := r.GetSettings(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.False(t, s.AIStyleCheckEnabled,
		"AIStyleCheckEnabled must reflect the DB-stored value")
}

// TestUseCase_UpdateSettings_AcceptsAIStyleCheckEnabled asserts that the
// use case forwards the toggle to the repository fields map when the API
// request includes "ai_style_check_enabled". Without an entry in the
// UseCase's raw-map switch this column never reaches UPDATE SET.
func TestUseCase_UpdateSettings_AcceptsAIStyleCheckEnabled(t *testing.T) {
	repo := &mockSettingsRepo{
		settings: &domain.Settings{AIStyleCheckEnabled: true},
	}
	uc := NewUseCase(repo, &mockTelegramValidator{})

	raw := map[string]json.RawMessage{
		"ai_style_check_enabled": json.RawMessage("false"),
	}
	input := SettingsInput{AIStyleCheckEnabled: false}

	_, err := uc.UpdateSettings(context.Background(), uuid.New(), raw, input)
	require.NoError(t, err)
	require.Contains(t, repo.updated, "ai_style_check_enabled",
		"use case must forward ai_style_check_enabled into the fields map")
	assert.Equal(t, false, repo.updated["ai_style_check_enabled"])
}
