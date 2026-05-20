package httputil

import (
	"net/http"
	"strings"
)

// DefaultMaxBodyBytes is the absolute ceiling applied by MaxBodyBytes.
// 10 MiB comfortably accommodates legitimate multipart uploads (CSV
// imports etc.) while still bounding the worst-case Content-Length a
// single request can stream into a handler — the outer floor in the
// defence-in-depth stack with JSONBodyCap.
const DefaultMaxBodyBytes int64 = 10 * 1024 * 1024

// DefaultMaxJSONBodyBytes is the default cap applied by JSONBodyCap.
// 1 MiB is far beyond any structured JSON payload Floq accepts and far
// below the DoS threshold. The inner (tighter) layer in the
// defence-in-depth stack — only fires on application/json bodies.
const DefaultMaxJSONBodyBytes int64 = 1 * 1024 * 1024

// MaxBodyBytes returns middleware that wraps r.Body in
// http.MaxBytesReader unconditionally — regardless of Content-Type.
// Use this as the outer ceiling so a client that omits or spoofs
// Content-Type cannot stream past the cap into a handler that then
// calls json.NewDecoder.Decode or io.ReadAll.
//
// Tighter content-type-specific caps (e.g. JSONBodyCap) compose with
// this middleware: MaxBytesReader honours the smallest cap in the
// chain, so the inner layer fires first on JSON traffic.
func MaxBodyBytes(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// JSONBodyCap returns middleware that wraps r.Body in
// http.MaxBytesReader when the request Content-Type is
// application/json (with or without a charset suffix). Multipart and
// other non-JSON content types are untouched — file upload routes
// carry their own size considerations and must opt in separately.
//
// Handlers needing a tighter local cap can still wrap r.Body again;
// MaxBytesReader honours the smallest cap in the chain.
func JSONBodyCap(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && isJSONContentType(r.Header.Get("Content-Type")) {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isJSONContentType(ct string) bool {
	if ct == "" {
		return false
	}
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.EqualFold(strings.TrimSpace(ct), "application/json")
}
