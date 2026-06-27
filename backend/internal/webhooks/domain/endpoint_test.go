package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNewWebhookURL_Valid(t *testing.T) {
	cases := []string{
		"https://hooks.zapier.com/abc",
		"http://example.com/webhook",
		"https://example.com:8443/path?q=1",
		"  https://example.com/x  ", // trimmed
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			u, err := NewWebhookURL(in)
			if err != nil {
				t.Fatalf("NewWebhookURL(%q): unexpected error %v", in, err)
			}
			if u.String() == "" {
				t.Fatal("NewWebhookURL produced empty URL")
			}
		})
	}
}

// SSRF defense layer 1: the VO rejects IP-literal hosts and non-HTTP schemes at
// construction, before the request is ever built. Layer 2 (the dial guard on
// the resolved IP) is in the delivery client.
func TestNewWebhookURL_RejectsSSRFVectors(t *testing.T) {
	cases := []string{
		"",                              // empty
		"ftp://example.com",             // non-http scheme
		"file:///etc/passwd",            // file scheme
		"https://127.0.0.1/x",           // loopback IP literal
		"http://169.254.169.254/latest", // cloud metadata IP literal
		"https://10.0.0.5/x",            // private IP literal
		"https://[::1]/x",               // IPv6 loopback literal
		"https://localhost/x",           // localhost hostname
		"http://user:pass@example.com",  // userinfo (credential smuggling)
		"https:///nohost",               // missing host
		"not a url",                     // unparseable
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			if _, err := NewWebhookURL(in); !errors.Is(err, ErrInvalidWebhookURL) {
				t.Fatalf("NewWebhookURL(%q): want ErrInvalidWebhookURL, got %v", in, err)
			}
		})
	}
}

func TestNewWebhookEndpoint_Valid(t *testing.T) {
	userID := uuid.New()
	ep, err := NewWebhookEndpoint(userID, "https://example.com/hook",
		[]EventType{EventLeadCreated, EventLeadQualified}, "supersecretvalue123")
	if err != nil {
		t.Fatalf("NewWebhookEndpoint: unexpected error %v", err)
	}
	if ep.ID == uuid.Nil {
		t.Error("endpoint must have a generated ID")
	}
	if ep.UserID != userID {
		t.Error("endpoint must carry the owner user ID")
	}
	if !ep.Active {
		t.Error("a new endpoint should be active by default")
	}
	if !ep.Subscribes(EventLeadCreated) || ep.Subscribes(EventSequenceCompleted) {
		t.Error("Subscribes must reflect the configured event set")
	}
	if ep.Secret == "" {
		t.Error("secret must be retained for signing")
	}
}

func TestNewWebhookEndpoint_Invariants(t *testing.T) {
	userID := uuid.New()
	cases := []struct {
		name   string
		url    string
		events []EventType
		secret string
		want   error
	}{
		{"nil user", "https://x.com/h", []EventType{EventLeadCreated}, "secret12345678", ErrEmptyOwner},
		{"bad url", "ftp://x.com", []EventType{EventLeadCreated}, "secret12345678", ErrInvalidWebhookURL},
		{"no events", "https://x.com/h", nil, "secret12345678", ErrNoEvents},
		{"unknown event", "https://x.com/h", []EventType{EventType("lead.boom")}, "secret12345678", ErrUnknownEventType},
		{"short secret", "https://x.com/h", []EventType{EventLeadCreated}, "short", ErrWeakSecret},
		{"dup events ok dedups", "https://x.com/h", []EventType{EventLeadCreated, EventLeadCreated}, "secret12345678", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			uid := userID
			if c.name == "nil user" {
				uid = uuid.Nil
			}
			ep, err := NewWebhookEndpoint(uid, c.url, c.events, c.secret)
			if c.want == nil {
				if err != nil {
					t.Fatalf("unexpected error %v", err)
				}
				// dedup: two identical events collapse to one.
				if len(ep.Events) != 1 {
					t.Fatalf("expected deduped events len 1, got %d", len(ep.Events))
				}
				return
			}
			if !errors.Is(err, c.want) {
				t.Fatalf("want %v, got %v", c.want, err)
			}
		})
	}
}
