package domain

import (
	"errors"
	"net"
	"net/url"
	"slices"
	"strings"

	"github.com/google/uuid"
)

// Domain errors. Declared as package vars so callers match with errors.Is and
// the HTTP layer can map them to status codes without string matching.
var (
	ErrInvalidWebhookURL = errors.New("webhooks: invalid or unsafe URL")
	ErrEmptyOwner        = errors.New("webhooks: endpoint must have an owner")
	ErrNoEvents          = errors.New("webhooks: endpoint must subscribe to at least one event")
	ErrWeakSecret        = errors.New("webhooks: secret too short")
)

// minSecretLen is the floor for a signing secret. Short secrets weaken the HMAC
// against brute force; receivers can generate any longer string.
const minSecretLen = 16

// WebhookURL is a value object: a delivery target proven safe at construction.
// This is SSRF defense layer 1 — it rejects non-HTTP schemes, embedded
// credentials, missing hosts, and IP-literal / localhost targets so a
// subscription can never be created pointing at an internal address. Layer 2
// (the dial guard on the resolved IP, defeating DNS rebinding) lives in the
// delivery client.
type WebhookURL struct {
	raw string
}

// NewWebhookURL validates and normalizes a delivery URL.
func NewWebhookURL(raw string) (WebhookURL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return WebhookURL{}, ErrInvalidWebhookURL
	}
	u, err := url.Parse(raw)
	if err != nil {
		return WebhookURL{}, ErrInvalidWebhookURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return WebhookURL{}, ErrInvalidWebhookURL
	}
	if u.User != nil {
		// Embedded credentials (user:pass@host) are a request-smuggling /
		// credential-leak vector and never legitimate for a webhook target.
		return WebhookURL{}, ErrInvalidWebhookURL
	}
	host := u.Hostname()
	if host == "" {
		return WebhookURL{}, ErrInvalidWebhookURL
	}
	// Reject IP-literal hosts outright: a subscription should name a real
	// hostname. An IP literal is the classic SSRF target (loopback, link-local
	// metadata 169.254.169.254, private ranges) and bypasses the intent of a
	// public webhook endpoint.
	if ip := net.ParseIP(host); ip != nil {
		return WebhookURL{}, ErrInvalidWebhookURL
	}
	// Block obvious localhost aliases by name too (resolution-independent).
	if isBlockedHostname(host) {
		return WebhookURL{}, ErrInvalidWebhookURL
	}
	return WebhookURL{raw: u.String()}, nil
}

// isBlockedHostname rejects hostnames that resolve to the local machine
// regardless of DNS. The resolved-IP guard in the delivery client is the
// authoritative defense; this just stops the most obvious mistakes at config
// time with a clear error.
func isBlockedHostname(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	return h == "localhost" || h == "localhost.localdomain" || strings.HasSuffix(h, ".localhost")
}

// String returns the normalized URL.
func (u WebhookURL) String() string { return u.raw }

// WebhookURLFromStorage reconstructs a URL from a trusted, previously-validated
// stored value without re-running validation (mirrors DomainFromStorage in the
// enrichment context). Use only for hydration from the database.
func WebhookURLFromStorage(raw string) WebhookURL { return WebhookURL{raw: raw} }

// WebhookEndpoint is an aggregate: a subscription owned by a user, targeting a
// safe URL, listening for a non-empty set of known events, signed with a secret.
type WebhookEndpoint struct {
	ID     uuid.UUID
	UserID uuid.UUID
	URL    WebhookURL
	Events []EventType
	Secret string
	Active bool
}

// NewWebhookEndpoint constructs a valid, active endpoint, enforcing every
// invariant: real owner, safe URL, at least one known (deduped) event, and a
// secret of adequate length.
func NewWebhookEndpoint(userID uuid.UUID, rawURL string, events []EventType, secret string) (*WebhookEndpoint, error) {
	if userID == uuid.Nil {
		return nil, ErrEmptyOwner
	}
	u, err := NewWebhookURL(rawURL)
	if err != nil {
		return nil, err
	}
	deduped, err := normalizeEvents(events)
	if err != nil {
		return nil, err
	}
	if len(secret) < minSecretLen {
		return nil, ErrWeakSecret
	}
	return &WebhookEndpoint{
		ID:     uuid.New(),
		UserID: userID,
		URL:    u,
		Events: deduped,
		Secret: secret,
		Active: true,
	}, nil
}

// normalizeEvents validates each event against the registry and removes
// duplicates while preserving first-seen order. An empty input is an error.
func normalizeEvents(events []EventType) ([]EventType, error) {
	if len(events) == 0 {
		return nil, ErrNoEvents
	}
	seen := make(map[EventType]struct{}, len(events))
	out := make([]EventType, 0, len(events))
	for _, e := range events {
		if !e.IsKnown() {
			return nil, ErrUnknownEventType
		}
		if _, dup := seen[e]; dup {
			continue
		}
		seen[e] = struct{}{}
		out = append(out, e)
	}
	return out, nil
}

// Subscribes reports whether this endpoint listens for et.
func (e *WebhookEndpoint) Subscribes(et EventType) bool {
	return slices.Contains(e.Events, et)
}
