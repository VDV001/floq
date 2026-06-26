package enrichment

import (
	"context"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/daniil/floq/internal/enrichment/domain"
)

// HTMLExtractor is the Phase-1 Extractor: a pure, deterministic parser that
// pulls a CompanyProfile out of raw page HTML using regexes — no LLM, no I/O.
// The Extractor port lets a Phase-2 LLMExtractor slot in behind the same seam.
type HTMLExtractor struct{}

// NewHTMLExtractor constructs the HTML extractor.
func NewHTMLExtractor() *HTMLExtractor { return &HTMLExtractor{} }

var (
	titleRe    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	metaRe     = regexp.MustCompile(`(?is)<meta\b[^>]*>`)
	contentRe  = regexp.MustCompile(`(?is)content\s*=\s*["']([^"']*)["']`)
	descNameRe = regexp.MustCompile(`(?is)(?:name|property)\s*=\s*["']description["']`)
	ogDescRe   = regexp.MustCompile(`(?is)(?:name|property)\s*=\s*["']og:description["']`)
	wsRe       = regexp.MustCompile(`\s+`)

	emailRe   = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	mailtoRe  = regexp.MustCompile(`(?i)href\s*=\s*["']mailto:([^"'?]+)`)
	telRe     = regexp.MustCompile(`(?i)href\s*=\s*["']tel:([^"']+)["']`)
	hrefRe    = regexp.MustCompile(`(?i)href\s*=\s*["']([^"']+)["']`)
	telDigits = regexp.MustCompile(`[^\d+]`)
)

// emailJunkPrefixes are local-part prefixes that are never a real contact.
var emailJunkPrefixes = []string{"noreply", "no-reply", "no_reply", "mailer-daemon", "postmaster", "webmaster"}

// socialHosts are the hosts (after stripping a leading www.) treated as social
// profile links worth surfacing.
var socialHosts = map[string]struct{}{
	"t.me": {}, "telegram.me": {}, "vk.com": {}, "linkedin.com": {},
	"facebook.com": {}, "fb.com": {}, "instagram.com": {},
	"youtube.com": {}, "youtu.be": {}, "wa.me": {},
}

// Extract parses page HTML into a CompanyProfile. It never errors on bad input
// — an unparseable page yields an empty (IsEmpty) profile.
func (HTMLExtractor) Extract(_ context.Context, page string) (domain.CompanyProfile, error) {
	return domain.CompanyProfile{
		Title:       extractTitle(page),
		Description: extractDescription(page),
		Emails:      extractEmails(page),
		Phones:      extractPhones(page),
		Socials:     extractSocials(page),
	}, nil
}

func extractTitle(page string) string {
	m := titleRe.FindStringSubmatch(page)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(wsRe.ReplaceAllString(m[1], " "))
}

func extractDescription(page string) string {
	var og string
	for _, tag := range metaRe.FindAllString(page, -1) {
		c := contentRe.FindStringSubmatch(tag)
		if len(c) < 2 {
			continue
		}
		content := strings.TrimSpace(c[1])
		if descNameRe.MatchString(tag) {
			return content // plain description wins outright
		}
		if og == "" && ogDescRe.MatchString(tag) {
			og = content
		}
	}
	return og
}

func extractEmails(page string) []string {
	seen := map[string]struct{}{}
	add := func(raw string) {
		e := strings.ToLower(strings.TrimSpace(raw))
		if e == "" || isJunkEmail(e) {
			return
		}
		seen[e] = struct{}{}
	}
	for _, m := range mailtoRe.FindAllStringSubmatch(page, -1) {
		add(m[1])
	}
	for _, m := range emailRe.FindAllString(page, -1) {
		add(m)
	}
	return sortedKeys(seen)
}

func isJunkEmail(email string) bool {
	local := email
	if i := strings.IndexByte(email, '@'); i >= 0 {
		local = email[:i]
	}
	for _, p := range emailJunkPrefixes {
		if strings.HasPrefix(local, p) {
			return true
		}
	}
	return false
}

func extractPhones(page string) []string {
	seen := map[string]struct{}{}
	for _, m := range telRe.FindAllStringSubmatch(page, -1) {
		norm := telDigits.ReplaceAllString(m[1], "")
		if norm == "" {
			continue
		}
		// Keep a leading + only if it was the first character.
		if strings.Contains(norm[1:], "+") {
			norm = norm[:1] + strings.ReplaceAll(norm[1:], "+", "")
		}
		if len(strings.Trim(norm, "+")) >= 7 {
			seen[norm] = struct{}{}
		}
	}
	return sortedKeys(seen)
}

func extractSocials(page string) []string {
	seen := map[string]struct{}{}
	for _, m := range hrefRe.FindAllStringSubmatch(page, -1) {
		raw := strings.TrimSpace(m[1])
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			continue
		}
		host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
		if _, ok := socialHosts[host]; ok {
			seen[raw] = struct{}{}
		}
	}
	return sortedKeys(seen)
}

func sortedKeys(m map[string]struct{}) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
