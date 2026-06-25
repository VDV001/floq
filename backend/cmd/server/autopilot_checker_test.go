package main

import (
	"context"
	"errors"
	"testing"
	"time"

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

// The adapter maps AutoSend → Enabled and AutoSendDelayMin → SendDelay. A
// negative delay is clamped to zero (never schedule a send in the past).
func TestAutopilotCheckerAdapter(t *testing.T) {
	tests := []struct {
		name        string
		settings    *settingsdomain.Settings
		wantEnabled bool
		wantDelay   time.Duration
	}{
		{"autosend on, 5 min delay", &settingsdomain.Settings{AutoSend: true, AutoSendDelayMin: 5}, true, 5 * time.Minute},
		{"autosend off", &settingsdomain.Settings{AutoSend: false}, false, 0},
		{"negative delay clamped", &settingsdomain.Settings{AutoSend: true, AutoSendDelayMin: -3}, true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := autopilotCheckerAdapter{settings: fakeSettingsStore{settings: tt.settings}}
			got, err := a.ResolveAutopilot(context.Background(), uuid.New())
			require.NoError(t, err)
			assert.Equal(t, tt.wantEnabled, got.Enabled)
			assert.Equal(t, tt.wantDelay, got.SendDelay)
		})
	}
}

// A settings read failure must propagate as an error (not a silent disable) so
// the launch fails loudly instead of guessing the send mode.
func TestAutopilotCheckerAdapter_StoreErrorPropagates(t *testing.T) {
	a := autopilotCheckerAdapter{settings: fakeSettingsStore{err: errors.New("db down")}}
	got, err := a.ResolveAutopilot(context.Background(), uuid.New())
	require.Error(t, err)
	assert.False(t, got.Enabled, "must not report autopilot on when the setting is unreadable")
}
