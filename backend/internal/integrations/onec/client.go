package onec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/daniil/floq/internal/integrations/onec/domain"
)

// counterpartyCatalogPath is the OData resource for counterparties. The
// connector is universal (the concrete 1C config is out of scope, #108), so we
// target the standard catalog name with a JSON format hint; install-specific
// field/catalog overrides are deferred.
const counterpartyCatalogPath = "/Catalog_Контрагенты"

// clientMaxAttempts caps retries at 3 — first try + two backoffs. Mirrors the
// outbound email sender: enough to ride out a transient 5xx, not so many that a
// caller blocks behind a sticky failure.
const clientMaxAttempts = 3

// clientInitialBackoff is the wait before the second attempt, doubled for the
// third (200ms → 400ms), matching internal/outbound.
const clientInitialBackoff = 200 * time.Millisecond

// HTTPClient is the HTTP/OData implementation of OneCClient. It is stateless
// per tenant — credentials are passed per call — so a single instance serves
// every user.
type HTTPClient struct {
	httpClient *http.Client
	backoff    time.Duration
}

// HTTPClientOption configures an HTTPClient.
type HTTPClientOption func(*HTTPClient)

// WithClientBackoff overrides the initial retry backoff (tests shrink it so the
// retry paths don't add real wall-clock delay).
func WithClientBackoff(d time.Duration) HTTPClientOption {
	return func(c *HTTPClient) { c.backoff = d }
}

// NewHTTPClient builds an HTTPClient over hc (falls back to http.DefaultClient
// when nil).
func NewHTTPClient(hc *http.Client, opts ...HTTPClientOption) *HTTPClient {
	c := &HTTPClient{httpClient: hc, backoff: clientInitialBackoff}
	if c.httpClient == nil {
		c.httpClient = http.DefaultClient
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// counterpartyPayload is the OData create body. Field names follow the standard
// 1C catalog; the universal connector keeps them fixed (per-field mapping is
// out of scope for #108).
type counterpartyPayload struct {
	Description string `json:"Description"`
	Email       string `json:"Email"`
	Company     string `json:"Company"`
}

// createResponse is the slice of the OData create response we read back: the
// reference 1C assigns to the new object.
type createResponse struct {
	RefKey string `json:"Ref_Key"`
}

// CreateCounterparty POSTs a counterparty to 1C and returns its Ref_Key.
// Retries transport errors and 5xx with exponential backoff; a 4xx is terminal
// (the same body cannot succeed on retry). The error wraps the status so the
// caller can record it in the ledger.
func (c *HTTPClient) CreateCounterparty(ctx context.Context, creds *domain.OutboundCredentials, draft *domain.CounterpartyDraft) (string, error) {
	body, err := json.Marshal(counterpartyPayload{
		Description: draft.Name,
		Email:       draft.Email,
		Company:     draft.Company,
	})
	if err != nil {
		return "", fmt.Errorf("onec: marshal counterparty: %w", err)
	}
	url := creds.BaseURL + counterpartyCatalogPath + "?$format=json"

	var ref string
	err = c.doRetrying(ctx, http.MethodPost, url, body, creds, func(resp *http.Response) (bool, error) {
		r, retry, hErr := handleCreateResponse(resp)
		ref = r
		return retry, hErr
	})
	return ref, err
}

// reconcileEventsPath is the 1C HTTP-service Floq polls for recent events during
// reconciliation. Like the write path, the concrete 1C config is out of scope —
// the endpoint returns events already in Floq's canonical shape.
const reconcileEventsPath = "/floq_events"

// eventDTO is one event as returned by the reconcile read endpoint. Kind is
// optional (derived from mapping when empty); Payload is the raw 1C body.
type eventDTO struct {
	ExternalID   string          `json:"external_id"`
	ExternalType string          `json:"external_type"`
	Kind         string          `json:"kind"`
	Payload      json.RawMessage `json:"payload"`
}

// ListEvents GETs the tenant's recent 1C events for reconciliation. Retries
// transport errors and 5xx; a 4xx is terminal.
func (c *HTTPClient) ListEvents(ctx context.Context, creds *domain.OutboundCredentials) ([]RawInboundEvent, error) {
	url := creds.BaseURL + reconcileEventsPath + "?$format=json"

	var events []RawInboundEvent
	err := c.doRetrying(ctx, http.MethodGet, url, nil, creds, func(resp *http.Response) (bool, error) {
		evs, retry, hErr := handleListResponse(resp)
		events = evs
		return retry, hErr
	})
	if err != nil {
		return nil, err
	}
	return events, nil
}

// doRetrying issues method+url with creds auth, retrying transport errors and
// 5xx with exponential backoff. handle classifies each response: it returns
// (retry, err) — a nil err stops successfully, a non-nil err with retry=false is
// terminal (4xx), retry=true loops until attempts are exhausted.
func (c *HTTPClient) doRetrying(ctx context.Context, method, url string, body []byte, creds *domain.OutboundCredentials, handle func(*http.Response) (bool, error)) error {
	var lastErr error
	backoff := c.backoff
	for attempt := 1; attempt <= clientMaxAttempts; attempt++ {
		// New Request per attempt — bytes.Reader's cursor is consumed by Do.
		var reqBody io.Reader
		if body != nil {
			reqBody = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			return fmt.Errorf("onec: build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		setAuth(req, creds)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("onec: %s %s (attempt %d): %w", method, url, attempt, err)
		} else {
			retry, hErr := handle(resp)
			if hErr == nil {
				return nil
			}
			lastErr = hErr
			if !retry {
				return lastErr // terminal (4xx)
			}
		}

		if attempt < clientMaxAttempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
		}
	}
	return lastErr
}

// handleListResponse classifies a reconcile read response, parsing the event
// array on 2xx. Signature mirrors handleCreateResponse.
func handleListResponse(resp *http.Response) ([]RawInboundEvent, bool, error) {
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		var dtos []eventDTO
		if err := json.Unmarshal(raw, &dtos); err != nil {
			return nil, false, fmt.Errorf("onec: decode events: %w", err)
		}
		events := make([]RawInboundEvent, len(dtos))
		for i, d := range dtos {
			events[i] = RawInboundEvent{
				ExternalID:   d.ExternalID,
				ExternalType: d.ExternalType,
				Kind:         d.Kind,
				Payload:      d.Payload,
			}
		}
		return events, false, nil
	case resp.StatusCode >= 500:
		return nil, true, fmt.Errorf("onec: 1C returned %d: %s", resp.StatusCode, snippet(raw))
	default:
		return nil, false, fmt.Errorf("onec: 1C rejected read %d: %s", resp.StatusCode, snippet(raw))
	}
}

// setAuth attaches the tenant credential per its auth type.
func setAuth(req *http.Request, creds *domain.OutboundCredentials) {
	switch creds.AuthType {
	case domain.AuthTypeToken:
		req.Header.Set("Authorization", "Bearer "+creds.AuthSecret)
	default: // basic
		req.Header.Set("Authorization", "Basic "+creds.AuthSecret)
	}
}

// handleCreateResponse classifies a response: (ref, _, nil) on 2xx,
// (_, true, err) on a retryable 5xx/transport-ish status, (_, false, err) on a
// terminal 4xx. It always drains and closes the body.
func handleCreateResponse(resp *http.Response) (string, bool, error) {
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		var cr createResponse
		_ = json.Unmarshal(raw, &cr) // empty/non-JSON body → empty ref, still success
		return cr.RefKey, false, nil
	case resp.StatusCode >= 500:
		return "", true, fmt.Errorf("onec: 1C returned %d: %s", resp.StatusCode, snippet(raw))
	default: // 4xx and other non-2xx
		return "", false, fmt.Errorf("onec: 1C rejected request %d: %s", resp.StatusCode, snippet(raw))
	}
}

// snippet trims a response body for error messages.
func snippet(b []byte) string {
	const max = 200
	if len(b) > max {
		return string(b[:max])
	}
	return string(b)
}
