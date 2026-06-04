package ratelimit

import (
	"net"
	"net/http"
	"strings"
)

// IPFromRequest resolves the client IP of an unauthenticated request so
// public endpoints (login, register) can be bucketed per source address.
// Precedence: X-Forwarded-For (first hop) → X-Real-IP → RemoteAddr. It
// returns ok=false when no usable IP can be extracted, which the
// middleware treats as "do not rate-limit" (fail-open) rather than
// hashing every unresolvable request into one shared bucket.
//
// SECURITY: X-Forwarded-For / X-Real-IP are client-supplied and trivially
// spoofable. Trusting them is only safe when the app sits behind a
// reverse proxy that OVERWRITES these headers with the real peer address
// (the deploy topology here — chi behind a proxy). If the app is ever
// exposed directly, an attacker can rotate the header to dodge the
// per-IP cap; in that topology this resolver should be gated behind a
// trust-proxy switch. Documented rather than silently assumed.
func IPFromRequest(r *http.Request) (string, bool) {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First hop is the originating client; the rest are proxies.
		first := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
		if first != "" {
			return first, true
		}
	}
	if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
		return xr, true
	}
	if r.RemoteAddr == "" {
		return "", false
	}
	// RemoteAddr is normally host:port; tolerate a bare IP too.
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		if host != "" {
			return host, true
		}
		return "", false
	}
	if ip := net.ParseIP(r.RemoteAddr); ip != nil {
		return r.RemoteAddr, true
	}
	return "", false
}

// IPKeyFunc returns a KeyFunc that buckets requests by client IP under
// the given namespace prefix. The prefix keeps separate limiters (e.g.
// login vs register) from colliding in a shared Redis keyspace, where a
// bare IP key would otherwise mix attempts across both endpoints. When
// the IP cannot be resolved it returns ok=false so the middleware
// fails open.
func IPKeyFunc(prefix string) KeyFunc {
	return func(r *http.Request) (string, bool) {
		ip, ok := IPFromRequest(r)
		if !ok {
			return "", false
		}
		return prefix + ip, true
	}
}
