package enrichment

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	fetchUserAgent = "Mozilla/5.0 (compatible; FloqBot/1.0)"
	fetchMaxBytes  = 1 << 20 // 1 MiB cap per page
	fetchTimeout   = 15 * time.Second
)

// WebsiteFetcher implements PageFetcher by fetching a company's homepage over
// the shared proxy-aware HTTP client.
type WebsiteFetcher struct {
	client *http.Client
}

// NewWebsiteFetcher builds the fetcher. A nil client falls back to a default
// one with a sane timeout.
func NewWebsiteFetcher(client *http.Client) *WebsiteFetcher {
	if client == nil {
		client = &http.Client{Timeout: fetchTimeout}
	}
	return &WebsiteFetcher{client: client}
}

var _ PageFetcher = (*WebsiteFetcher)(nil)

// Fetch retrieves the homepage HTML for a company domain over HTTPS.
func (f *WebsiteFetcher) Fetch(ctx context.Context, domainName string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	return fetchPage(ctx, f.client, "https://"+domainName, fetchMaxBytes)
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
