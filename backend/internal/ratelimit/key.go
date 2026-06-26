package ratelimit

import (
	"net"
	"net/http"
	"strings"
)

// IPFromRequest resolves the client IP of an unauthenticated request so
// public endpoints (login, register) can be bucketed per source address.
// It returns ok=false when no usable IP can be extracted, which the
// middleware treats as "do not rate-limit" (fail-open) rather than
// hashing every unresolvable request into one shared bucket.
//
// trustProxy decides whether forwarded headers are believed:
//
//   - false (default): use only RemoteAddr — the real TCP peer. This is
//     correct when the app is exposed directly (the committed
//     docker-compose binds backend :8080 with no reverse proxy). X-
//     Forwarded-For / X-Real-IP are client-supplied and trivially
//     spoofable, so a different value per request would let an attacker
//     dodge the per-IP cap entirely — exactly the brute-force the limit
//     exists to stop. Ignoring them closes that bypass.
//   - true: honor X-Forwarded-For (first hop) → X-Real-IP → RemoteAddr.
//     Only safe behind a reverse proxy that OVERWRITES these headers
//     with the real peer address. Enable via TRUST_PROXY once such a
//     proxy is in front of the app.
//
// Header values are validated with net.ParseIP; a malformed entry falls
// through to the next source rather than becoming a bogus bucket key.
func IPFromRequest(r *http.Request, trustProxy bool) (string, bool) {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// First hop is the originating client; the rest are proxies.
			first := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
			if net.ParseIP(first) != nil {
				return first, true
			}
		}
		if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); net.ParseIP(xr) != nil {
			return xr, true
		}
	}
	if r.RemoteAddr == "" {
		return "", false
	}
	// RemoteAddr is normally host:port; tolerate a bare IP too.
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		if net.ParseIP(host) != nil {
			return host, true
		}
		return "", false
	}
	if net.ParseIP(r.RemoteAddr) != nil {
		return r.RemoteAddr, true
	}
	return "", false
}

// IPKeyFunc returns a KeyFunc that buckets requests by client IP under
// the given namespace prefix. The prefix keeps separate limiters (e.g.
// login vs register) from colliding in a shared Redis keyspace, where a
// bare IP key would otherwise mix attempts across both endpoints.
// trustProxy is threaded to IPFromRequest. When the IP cannot be
// resolved it returns ok=false so the middleware fails open.
func IPKeyFunc(prefix string, trustProxy bool) KeyFunc {
	return func(r *http.Request) (string, bool) {
		ip, ok := IPFromRequest(r, trustProxy)
		if !ok {
			return "", false
		}
		return prefix + ip, true
	}
}
