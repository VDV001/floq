package outbound

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	prospectsdomain "github.com/daniil/floq/internal/prospects/domain"
	seqdomain "github.com/daniil/floq/internal/sequences/domain"
	settingsdomain "github.com/daniil/floq/internal/settings/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captured collects header + body of every request hitting the Resend
// stub so tests can pin both presence and stability of the
// Idempotency-Key across attempts.
type captured struct {
	mu      sync.Mutex
	headers []http.Header
}

func (c *captured) record(h http.Header) {
	c.mu.Lock()
	defer c.mu.Unlock()
	clone := h.Clone()
	c.headers = append(c.headers, clone)
}

func (c *captured) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.headers)
}

func (c *captured) idempotencyKeys() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	keys := make([]string, 0, len(c.headers))
	for _, h := range c.headers {
		keys = append(keys, h.Get("Idempotency-Key"))
	}
	return keys
}

// stubResend swaps the package-level resendAPIURL for a httptest server
// driven by handler. Returns the captured tape and a cleanup func.
func stubResend(t *testing.T, handler http.HandlerFunc) (*captured, func()) {
	t.Helper()
	c := &captured{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.record(r.Header)
		handler(w, r)
	}))
	old := resendAPIURL
	resendAPIURL = server.URL
	cleanup := func() {
		resendAPIURL = old
		server.Close()
	}
	return c, cleanup
}

func TestSendViaResend_SendsIdempotencyKeyHeader(t *testing.T) {
	c, cleanup := stubResend(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":"msg_x"}`)
	})
	defer cleanup()

	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}
	s := NewSender(cfgStore, uuid.New(), "test-key", "from@test.com", "",
		"", "", "", "",
		nil, nil, nil, nil, nil, nil)

	err := s.sendViaResend(context.Background(), "to@test.com", "subj", "body", "outbound:abc-123")
	require.NoError(t, err)

	keys := c.idempotencyKeys()
	require.Len(t, keys, 1, "exactly one Resend POST expected")
	assert.Equal(t, "outbound:abc-123", keys[0],
		"every Resend POST must carry the supplied Idempotency-Key verbatim — Resend dedups retries by this header")
}

func TestSendViaResend_RetriesOn5xxAndSucceeds(t *testing.T) {
	// First attempt: 503 (transient). Second: 200. With a stable
	// Idempotency-Key on both attempts, Resend dedups server-side —
	// our retry is safe. Test pins: success returned, exactly 2 calls,
	// same key on both.
	attempt := 0
	c, cleanup := stubResend(t, func(w http.ResponseWriter, _ *http.Request) {
		attempt++
		if attempt == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":"msg_x"}`)
	})
	defer cleanup()

	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}
	s := NewSender(cfgStore, uuid.New(), "test-key", "from@test.com", "",
		"", "", "", "",
		nil, nil, nil, nil, nil, nil)

	err := s.sendViaResend(context.Background(), "to@test.com", "subj", "body", "outbound:retry-1")
	require.NoError(t, err, "second attempt must succeed; retry loop must absorb the transient 503")
	assert.Equal(t, 2, c.calls(), "expected exactly 2 attempts (1 fail + 1 success)")

	keys := c.idempotencyKeys()
	require.Len(t, keys, 2)
	assert.Equal(t, keys[0], keys[1],
		"Idempotency-Key MUST be stable across retry attempts — otherwise Resend cannot dedup and we lose the safety we just added")
	assert.Equal(t, "outbound:retry-1", keys[0])
}

func TestSendViaResend_StopsAfterMaxAttemptsOn5xx(t *testing.T) {
	c, cleanup := stubResend(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}
	s := NewSender(cfgStore, uuid.New(), "test-key", "from@test.com", "",
		"", "", "", "",
		nil, nil, nil, nil, nil, nil)

	err := s.sendViaResend(context.Background(), "to@test.com", "subj", "body", "outbound:stuck")
	require.Error(t, err, "consecutive 5xx must surface after max attempts; retry cannot loop forever")

	got := c.calls()
	if got != 3 {
		t.Errorf("expected 3 attempts before giving up, got %d", got)
	}
}

func TestSendViaResend_DoesNotRetryOn4xx(t *testing.T) {
	// 4xx is a client error (bad payload, bad auth, etc.) — retrying
	// with the same Idempotency-Key and the same body cannot fix
	// the client mistake and just burns rate-limit budget on Resend.
	c, cleanup := stubResend(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})
	defer cleanup()

	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{}}
	s := NewSender(cfgStore, uuid.New(), "test-key", "from@test.com", "",
		"", "", "", "",
		nil, nil, nil, nil, nil, nil)

	err := s.sendViaResend(context.Background(), "to@test.com", "subj", "body", "outbound:bad")
	require.Error(t, err)
	assert.Equal(t, 1, c.calls(), "4xx must NOT retry — same body + same key cannot succeed on attempt 2")
}

func TestSendPending_UsesMessageIDAsIdempotencyKey(t *testing.T) {
	// SendPending end-to-end: drains one approved outbound row, calls
	// sendViaResend with an Idempotency-Key derived from the row ID.
	// Pins the wire contract: an attacker who guesses the next msg.ID
	// cannot replay; a retry of the same msg.ID hits Resend's dedup.
	c, cleanup := stubResend(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":"msg_x"}`)
	})
	defer cleanup()

	prospectID := uuid.New()
	msgID := uuid.New()

	seqRepo := &mockOutboundRepository{
		pending: []seqdomain.OutboundMessage{{
			ID:         msgID,
			ProspectID: prospectID,
			Channel:    seqdomain.StepChannelEmail,
			Status:     seqdomain.OutboundStatusApproved,
			Body:       "<p>hi</p>",
		}},
	}
	prospectRepo := &mockProspectLookup{
		prospects: map[uuid.UUID]*prospectsdomain.Prospect{
			prospectID: {ID: prospectID, Name: "Alice", Company: "Acme", Email: "alice@acme.com"},
		},
	}
	cfgStore := &mockConfigStore{cfg: &settingsdomain.UserConfig{ResendAPIKey: "test-key"}}

	s := NewSender(cfgStore, uuid.New(), "fallback-key", "from@test.com", "",
		"", "", "", "", // no SMTP — falls through to Resend
		seqRepo, prospectRepo, nil, nil, nil, nil)

	require.NoError(t, s.SendPending(context.Background()))
	require.Equal(t, 1, c.calls(), "expected exactly one Resend call for one pending row")

	keys := c.idempotencyKeys()
	require.Len(t, keys, 1)
	assert.True(t, strings.Contains(keys[0], msgID.String()),
		"Idempotency-Key %q must contain the message UUID so retries of the same row collapse server-side", keys[0])
}
