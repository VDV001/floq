package parser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

var mailtoRegex = regexp.MustCompile(`(?i)href\s*=\s*["']mailto:([^"'?]+)`)

var contactPaths = []string{
	"/contacts",
	"/contact",
	"/kontakty",
	"/about",
	"/about-us",
	"/o-kompanii",
}

var junkPrefixes = []string{
	"noreply",
	"no-reply",
	"no_reply",
	"mailer-daemon",
	"postmaster",
	"webmaster",
	"example",
	"test@",
	"admin@",
}

// ScrapeEmails fetches the target URL and its common contact pages,
// extracts email addresses from the HTML, deduplicates, filters junk,
// and returns up to 50 unique emails.
// If httpClient is nil, a default client is created.
func ScrapeEmails(targetURL string, httpClient *http.Client) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Normalize URL.
	if !strings.Contains(targetURL, "://") {
		targetURL = "https://" + targetURL
	}
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", targetURL, err)
	}

	if httpClient == nil {
		httpClient = &http.Client{}
	}
	client := &http.Client{
		Transport: httpClient.Transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("stopped after 3 redirects")
			}
			return nil
		},
	}

	seen := make(map[string]struct{})

	// Fetch main page.
	emails, err := fetchAndExtract(ctx, client, parsed.String())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch main page: %w", err)
	}
	for _, e := range emails {
		seen[strings.ToLower(e)] = struct{}{}
	}

	// Try contact page paths.
	baseURL := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
	for _, path := range contactPaths {
		pageURL := baseURL + path
		pageEmails, fetchErr := fetchAndExtract(ctx, client, pageURL)
		if fetchErr != nil {
			continue // silently skip errors
		}
		for _, e := range pageEmails {
			seen[strings.ToLower(e)] = struct{}{}
		}
	}

	// Collect, filter junk, enforce limit.
	var result []string
	for email := range seen {
		if isJunkEmail(email) {
			continue
		}
		result = append(result, email)
		if len(result) >= 50 {
			break
		}
	}

	return result, nil
}

func fetchAndExtract(ctx context.Context, client *http.Client, targetURL string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FloqBot/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	return extractEmails(string(body)), nil
}

func extractEmails(htmlBody string) []string {
	seen := make(map[string]struct{})
	var emails []string

	// Extract from mailto: links.
	for _, match := range mailtoRegex.FindAllStringSubmatch(htmlBody, -1) {
		if len(match) > 1 {
			email := strings.ToLower(strings.TrimSpace(match[1]))
			if _, ok := seen[email]; !ok {
				seen[email] = struct{}{}
				emails = append(emails, email)
			}
		}
	}

	// Extract from raw text via regex.
	for _, match := range emailRegex.FindAllString(htmlBody, -1) {
		email := strings.ToLower(strings.TrimSpace(match))
		if _, ok := seen[email]; !ok {
			seen[email] = struct{}{}
			emails = append(emails, email)
		}
	}

	return emails
}

func isJunkEmail(email string) bool {
	lower := strings.ToLower(email)
	for _, prefix := range junkPrefixes {
		if strings.Contains(prefix, "@") {
			// Prefix like "test@" or "admin@" — check if email starts with it.
			if strings.HasPrefix(lower, prefix) {
				return true
			}
		} else {
			// Prefix like "noreply" — check the local part before @.
			parts := strings.SplitN(lower, "@", 2)
			if len(parts) == 2 && strings.HasPrefix(parts[0], prefix) {
				return true
			}
		}
	}
	return false
}
