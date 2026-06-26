package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

const defaultTimeout = 30 * time.Second

// ContextDialer is an interface for dialers that support context.
type ContextDialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// Provider wraps a proxy URL and provides configured network primitives.
type Provider struct {
	dialer     ContextDialer
	httpClient *http.Client
}

// NewFromURL creates a Provider from a proxy URL string.
// Supported schemes: socks5://, http://, https://. Empty string means direct connection.
func NewFromURL(proxyURL string) (*Provider, error) {
	if proxyURL == "" {
		return &Provider{
			httpClient: &http.Client{Timeout: defaultTimeout},
		}, nil
	}

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	switch parsed.Scheme {
	case "socks5":
		return newSOCKS5Provider(parsed)
	case "http", "https":
		return newHTTPProvider(parsed)
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %q", parsed.Scheme)
	}
}

// Dialer returns a ContextDialer for TCP-level connections (SOCKS5 only).
// Returns nil for HTTP/HTTPS proxy or when no proxy is configured.
func (p *Provider) Dialer() ContextDialer {
	return p.dialer
}

// HTTPClient returns an *http.Client pre-configured with the proxy transport.
// Always returns non-nil.
func (p *Provider) HTTPClient() *http.Client {
	return p.httpClient
}

func newSOCKS5Provider(parsed *url.URL) (*Provider, error) {
	var auth *proxy.Auth
	if parsed.User != nil {
		pass, _ := parsed.User.Password()
		auth = &proxy.Auth{
			User:     parsed.User.Username(),
			Password: pass,
		}
	}

	host := parsed.Host
	dialer, err := proxy.SOCKS5("tcp", host, auth, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("creating SOCKS5 dialer: %w", err)
	}

	ctxDialer, ok := dialer.(ContextDialer)
	if !ok {
		return nil, fmt.Errorf("SOCKS5 dialer does not support DialContext")
	}

	transport := &http.Transport{
		DialContext: ctxDialer.DialContext,
	}

	return &Provider{
		dialer: ctxDialer,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   defaultTimeout,
		},
	}, nil
}

func newHTTPProvider(parsed *url.URL) (*Provider, error) {
	transport := &http.Transport{
		Proxy: http.ProxyURL(parsed),
	}

	return &Provider{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   defaultTimeout,
		},
	}, nil
}
