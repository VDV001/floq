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

// TestRepository_GetSettings_AggregatedInboxViewDefaultsTrue mirrors the
// style-check default test: when no user_settings row exists yet, the
// Go-side default and the SQL DEFAULT (migration 027) must agree on TRUE.
// A divergence here would silently flip every new user back to per-source
// view despite the documented opt-in default.
func TestRepository_GetSettings_AggregatedInboxViewDefaultsTrue(t *testing.T) {
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
			func() pgx.Row {
				return &fakeRow{scanFn: func(_ ...any) error { return pgx.ErrNoRows }}
			},
		},
	}

	r := NewRepositoryFromQuerier(q)
	s, err := r.GetSettings(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.True(t, s.AggregatedInboxView,
		"AggregatedInboxView must default to true when no user_settings row exists")
}

// TestRepository_GetSettings_AggregatedInboxViewFromDB asserts the DB
// value overrides the default. Scan index 25 corresponds to
// aggregated_inbox_view per the SELECT list (added after
// ai_style_check_enabled at index 24).
func TestRepository_GetSettings_AggregatedInboxViewFromDB(t *testing.T) {
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
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					if p, ok := dest[25].(*bool); ok {
						*p = false
					}
					return nil
				}}
			},
		},
	}

	r := NewRepositoryFromQuerier(q)
	s, err := r.GetSettings(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.False(t, s.AggregatedInboxView,
		"AggregatedInboxView must reflect the DB-stored value")
}

// TestUseCase_UpdateSettings_AcceptsAggregatedInboxView asserts the
// UpdateSettings whitelist forwards aggregated_inbox_view into the
// fields map. Without an entry, the column never reaches UPDATE SET
// and the toggle silently fails.
func TestUseCase_UpdateSettings_AcceptsAggregatedInboxView(t *testing.T) {
	repo := &mockSettingsRepo{
		settings: &domain.Settings{AggregatedInboxView: true},
	}
	uc := NewUseCase(repo, &mockTelegramValidator{})

	raw := map[string]json.RawMessage{
		"aggregated_inbox_view": json.RawMessage("false"),
	}
	input := SettingsInput{AggregatedInboxView: false}

	_, err := uc.UpdateSettings(context.Background(), uuid.New(), raw, input)
	require.NoError(t, err)
	require.Contains(t, repo.updated, "aggregated_inbox_view",
		"use case must forward aggregated_inbox_view into the fields map")
	assert.Equal(t, false, repo.updated["aggregated_inbox_view"])
}

// TestDomainToDTO_AggregatedInboxView asserts the handler mapper
// propagates the field — a missing line in domainToDTO would surface
// as a perpetual `true` to the UI regardless of the stored value.
func TestDomainToDTO_AggregatedInboxView(t *testing.T) {
	cases := []struct {
		name  string
		stored bool
	}{
		{"explicit true", true},
		{"explicit false", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dto := domainToDTO(&domain.Settings{AggregatedInboxView: tc.stored})
			assert.Equal(t, tc.stored, dto.AggregatedInboxView)
		})
	}
}
