package enrichment

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"
)

const (
	fetchUserAgent    = "Mozilla/5.0 (compatible; FloqBot/1.0)"
	fetchMaxBytes     = 1 << 20 // 1 MiB cap per page
	fetchTimeout      = 15 * time.Second
	fetchMaxRedirects = 3
)

// WebsiteFetcher implements PageFetcher by fetching a company's homepage over a
// dedicated, SSRF-hardened HTTP client.
type WebsiteFetcher struct {
	client *http.Client
}

// NewWebsiteFetcher builds the fetcher with its own egress-guarded client. It
// deliberately does NOT reuse the shared proxy-aware client: scraping an
// email-derived domain is an untrusted egress and must be dial-guarded against
// internal addresses (the shared client routes through the proxy, where the
// dial guard cannot see the resolved target IP).
func NewWebsiteFetcher() *WebsiteFetcher {
	return &WebsiteFetcher{client: newGuardedClient()}
}

var _ PageFetcher = (*WebsiteFetcher)(nil)

// Fetch retrieves the homepage HTML for a company domain over HTTPS.
func (f *WebsiteFetcher) Fetch(ctx context.Context, domainName string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	return fetchPage(ctx, f.client, "https://"+domainName, fetchMaxBytes)
}

// newGuardedClient builds an HTTP client whose dialer refuses to connect to
// loopback / private / link-local / unspecified addresses, evaluated on the
// RESOLVED IP (defeating DNS-rebinding). Redirects are capped and every hop is
// dialed through the same guard, so a redirect to an internal target is blocked
// too. This is SSRF defense layer 2 (layer 1 rejects IP-literal domains in the
// VO).
func newGuardedClient() *http.Client {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || isBlockedIP(ip) {
				return fmt.Errorf("enrichment: blocked egress to %q", address)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout:   fetchTimeout,
		Transport: &http.Transport{DialContext: dialer.DialContext},
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= fetchMaxRedirects {
				return fmt.Errorf("enrichment: stopped after %d redirects", fetchMaxRedirects)
			}
			return nil
		},
	}
}

// isBlockedIP reports whether ip is in a range the scraper must never reach.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

// fetchPage GETs rawURL with a bot User-Agent and returns up to maxBytes of the
// body. A 4xx/5xx response is an error; the body is hard-capped to bound memory.
func fetchPage(ctx context.Context, client *http.Client, rawURL string, maxBytes int64) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", fetchUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("enrichment: fetch %s: HTTP %d", rawURL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return "", err
	}
	return string(body), nil
}
