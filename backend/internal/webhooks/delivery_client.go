package webhooks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/daniil/floq/internal/webhooks/domain"
)

const (
	deliveryUserAgent = "FloqWebhooks/1.0"
	deliveryTimeout   = 15 * time.Second
	// respReadCap bounds how much of a receiver's response body we read. We only
	// need the status code; a misbehaving receiver must not be able to make us
	// buffer an unbounded body.
	respReadCap = 4 << 10 // 4 KiB
)

// HTTPDeliveryClient delivers webhook payloads over an SSRF-hardened HTTP client.
// It performs exactly ONE POST per call; retry/backoff is the outbox worker's
// job (the delivery row stays pending and is reclaimed), keeping retry semantics
// in one place.
type HTTPDeliveryClient struct {
	client *http.Client
}

var _ DeliveryClient = (*HTTPDeliveryClient)(nil)

// NewHTTPDeliveryClient builds the production client with the real egress guard.
func NewHTTPDeliveryClient() *HTTPDeliveryClient {
	return newHTTPDeliveryClientWithGuard(isBlockedIP, deliveryTimeout)
}

// newHTTPDeliveryClientWithGuard builds a client whose dialer refuses to connect
// to any IP for which blocked returns true, evaluated on the RESOLVED address
// (defeating DNS rebinding). The guard predicate is injectable so tests can
// exercise the happy path against a loopback-bound httptest server.
func newHTTPDeliveryClientWithGuard(blocked func(net.IP) bool, timeout time.Duration) *HTTPDeliveryClient {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || blocked(ip) {
				return fmt.Errorf("webhooks: blocked egress to %q", address)
			}
			return nil
		},
	}
	return &HTTPDeliveryClient{
		client: &http.Client{
			Timeout:   timeout,
			Transport: &http.Transport{DialContext: dialer.DialContext},
			// Never follow redirects: a 30x to an internal target would be an
			// SSRF pivot around the URL VO. Each hop would be dial-guarded too,
			// but refusing redirects outright is simpler and safer.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// isBlockedIP reports whether ip is in a range a webhook must never reach:
// loopback, private, link-local (incl. cloud metadata 169.254.169.254),
// unspecified, or multicast.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

// Deliver POSTs payload to url with the signature header. It returns the HTTP
// status (0 on transport failure) and a non-nil error for any transport problem
// or non-2xx response, so the worker records it and retries.
func (c *HTTPDeliveryClient) Deliver(ctx context.Context, url string, payload []byte, signature string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("webhooks: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", deliveryUserAgent)
	req.Header.Set(domain.SignatureHeader, signature)

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("webhooks: deliver: %w", err)
	}
	defer resp.Body.Close()
	// Drain a bounded amount so the connection can be reused; ignore the body.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, respReadCap))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("webhooks: non-2xx response: HTTP %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}
