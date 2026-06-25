package main

import (
	"context"
	"errors"
	"testing"

	settingsdomain "github.com/daniil/floq/internal/settings/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSettingsStore struct {
	settings *settingsdomain.Settings
	err      error
}

func (f fakeSettingsStore) GetSettings(_ context.Context, _ uuid.UUID) (*settingsdomain.Settings, error) {
	return f.settings, f.err
}

// The adapter reports autopilot enabled iff the user's AutoSend flag is set.
func TestAutopilotCheckerAdapter(t *testing.T) {
	tests := []struct {
		name     string
		settings *settingsdomain.Settings
		want     bool
	}{
		{"autosend on", &settingsdomain.Settings{AutoSend: true}, true},
		{"autosend off", &settingsdomain.Settings{AutoSend: false}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := autopilotCheckerAdapter{settings: fakeSettingsStore{settings: tt.settings}}
			got, err := a.IsAutopilotEnabled(context.Background(), uuid.New())
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// A settings read failure must propagate as an error (not a silent false) so
// the launch fails loudly instead of guessing the send mode.
func TestAutopilotCheckerAdapter_StoreErrorPropagates(t *testing.T) {
	a := autopilotCheckerAdapter{settings: fakeSettingsStore{err: errors.New("db down")}}
	on, err := a.IsAutopilotEnabled(context.Background(), uuid.New())
	require.Error(t, err)
	assert.False(t, on, "must not report autopilot on when the setting is unreadable")
}
