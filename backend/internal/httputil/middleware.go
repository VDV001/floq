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

// DefaultMaxUploadBytes is the ceiling for multipart CSV-import uploads
// (prospects/leads). Higher than DefaultMaxBodyBytes because legitimate
// enterprise imports can be large (≈50-100K rows); only the import routes get
// this looser cap, selected by path in MaxBodyBytesWithUploads.
const DefaultMaxUploadBytes int64 = 50 * 1024 * 1024

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

// MaxBodyBytesWithUploads is the outer body ceiling that lets CSV-import
// uploads exceed the general limit. It applies generalMax to every request,
// except paths ending in "/import" (the multipart upload routes) which get
// uploadMax.
//
// Why path-based and not a per-route middleware: http.MaxBytesReader is
// smallest-wins, so a higher inner cap on the import route cannot loosen a lower
// ancestor cap. The upload routes therefore must NOT sit under the general
// ceiling at all — selecting the cap by path here keeps both limits in one
// documented place instead of splitting the router. JSONBodyCap still composes
// on top, so JSON traffic trips its tighter 1 MiB cap first.
func MaxBodyBytesWithUploads(generalMax, uploadMax int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				limit := generalMax
				if strings.HasSuffix(r.URL.Path, "/import") {
					limit = uploadMax
				}
				r.Body = http.MaxBytesReader(w, r.Body, limit)
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
