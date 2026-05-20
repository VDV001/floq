package httputil

import (
	"net/http"
	"strings"
)

// DefaultMaxJSONBodyBytes is the default cap applied by JSONBodyCap.
// 1 MiB is far beyond any structured JSON payload Floq accepts and far
// below the DoS threshold (a malicious 1 GiB Content-Length would
// otherwise stream straight into json.NewDecoder.Decode).
const DefaultMaxJSONBodyBytes int64 = 1 * 1024 * 1024

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
