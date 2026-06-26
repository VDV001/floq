package main

import (
	"context"
	"errors"
	"testing"

	sequencesdomain "github.com/daniil/floq/internal/sequences/domain"
	settingsdomain "github.com/daniil/floq/internal/settings/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeConfigStore struct {
	cfg *settingsdomain.UserConfig
	err error
}

func (f fakeConfigStore) GetConfig(_ context.Context, _ uuid.UUID) (*settingsdomain.UserConfig, error) {
	return f.cfg, f.err
}

// The adapter resolves DB-then-env (mirroring the outbound sender) and treats
// email as configured iff a Resend key OR a full SMTP triple is present. A
// partial SMTP config (missing the password) is NOT enough.
func TestEmailConfigCheckerAdapter(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *settingsdomain.UserConfig
		adapter emailConfigCheckerAdapter // env fields only
		wantErr bool
	}{
		{"resend key in DB", &settingsdomain.UserConfig{ResendAPIKey: "re_x"}, emailConfigCheckerAdapter{}, false},
		{"full SMTP triple in DB", &settingsdomain.UserConfig{SMTPHost: "h", SMTPUser: "u", SMTPPassword: "p"}, emailConfigCheckerAdapter{}, false},
		{"partial SMTP is not enough", &settingsdomain.UserConfig{SMTPHost: "h", SMTPUser: "u"}, emailConfigCheckerAdapter{}, true},
		{"nothing configured", &settingsdomain.UserConfig{}, emailConfigCheckerAdapter{}, true},
		{"resend via env fallback", &settingsdomain.UserConfig{}, emailConfigCheckerAdapter{envResendKey: "re_env"}, false},
		{"SMTP via env fallback", &settingsdomain.UserConfig{}, emailConfigCheckerAdapter{envSMTPHost: "h", envSMTPUser: "u", envSMTPPassword: "p"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.adapter
			a.store = fakeConfigStore{cfg: tt.cfg}
			err := a.IsEmailConfigured(context.Background(), uuid.New())
			if tt.wantErr {
				require.ErrorIs(t, err, sequencesdomain.ErrEmailNotConfigured)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestEmailConfigCheckerAdapter_StoreErrorIsNotNotConfigured(t *testing.T) {
	a := emailConfigCheckerAdapter{store: fakeConfigStore{err: errors.New("db down")}}
	err := a.IsEmailConfigured(context.Background(), uuid.New())
	require.Error(t, err)
	// A store failure must not masquerade as "not configured" — the launch
	// handler would otherwise show the wrong remedy.
	assert.NotErrorIs(t, err, sequencesdomain.ErrEmailNotConfigured)
}
