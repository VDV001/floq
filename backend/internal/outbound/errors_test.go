package outbound

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	settingsdomain "github.com/daniil/floq/internal/settings/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendViaResend_NoAPIKey_ReturnsErrNoResendAPIKey(t *testing.T) {
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}
	s := NewSender(cfgStore, uuid.New(), "", "from@test.com", "",
		"", "", "", "",
		nil, nil, nil, nil, nil, nil)

	err := s.sendViaResend(context.Background(), "to@test.com", "subj", "body")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoResendAPIKey),
		"expected errors.Is to match ErrNoResendAPIKey, got: %v", err)
}

func TestSendViaResend_HTTP400_WrapsErrResendAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"message":"invalid sender"}`)
	}))
	defer server.Close()

	old := resendAPIURL
	resendAPIURL = server.URL
	defer func() { resendAPIURL = old }()

	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}
	s := NewSender(cfgStore, uuid.New(), "test-key", "from@test.com", "",
		"", "", "", "",
		nil, nil, nil, nil, nil, nil)

	err := s.sendViaResend(context.Background(), "to@test.com", "subj", "body")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrResendAPI),
		"expected errors.Is to match ErrResendAPI, got: %v", err)
}

func TestSendViaResend_HTTP500_WrapsErrResendAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	old := resendAPIURL
	resendAPIURL = server.URL
	defer func() { resendAPIURL = old }()

	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}
	s := NewSender(cfgStore, uuid.New(), "test-key", "from@test.com", "",
		"", "", "", "",
		nil, nil, nil, nil, nil, nil)

	err := s.sendViaResend(context.Background(), "to@test.com", "subj", "body")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrResendAPI),
		"expected errors.Is to match ErrResendAPI on 500, got: %v", err)
}

func TestSendViaResend_HTTP200_NoError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":"msg_123"}`)
	}))
	defer server.Close()

	old := resendAPIURL
	resendAPIURL = server.URL
	defer func() { resendAPIURL = old }()

	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}
	s := NewSender(cfgStore, uuid.New(), "test-key", "from@test.com", "",
		"", "", "", "",
		nil, nil, nil, nil, nil, nil)

	err := s.sendViaResend(context.Background(), "to@test.com", "subj", "body")
	assert.NoError(t, err)
}
