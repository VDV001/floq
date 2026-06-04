package httputil

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJSONBodyCap_AppliesToJSONContentType(t *testing.T) {
	// A handler that reads everything from r.Body and reports whether
	// the read errored — this is exactly the shape json.NewDecoder.Decode
	// produces when MaxBytesReader trips.
	var capturedErr error
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, capturedErr = io.ReadAll(r.Body)
	})

	mw := JSONBodyCap(10)(next)

	oversized := strings.Repeat("x", 50)
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(oversized))
	req.Header.Set("Content-Type", "application/json")
	mw.ServeHTTP(httptest.NewRecorder(), req)

	if capturedErr == nil {
		t.Fatal("expected ReadAll to error after exceeding cap, got nil")
	}
	var maxBytesErr *http.MaxBytesError
	if !errors.As(capturedErr, &maxBytesErr) {
		t.Fatalf("expected *http.MaxBytesError, got %T (%v)", capturedErr, capturedErr)
	}
}

func TestJSONBodyCap_PassesUnderCapJSON(t *testing.T) {
	var got string
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
	})

	mw := JSONBodyCap(100)(next)

	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(`{"ok":true}`))
	req.Header.Set("Content-Type", "application/json")
	mw.ServeHTTP(httptest.NewRecorder(), req)

	if got != `{"ok":true}` {
		t.Fatalf("body within cap must pass through unchanged, got %q", got)
	}
}

func TestJSONBodyCap_SkipsMultipartContentType(t *testing.T) {
	// Multipart routes (CSV import etc.) need their own cap reasoning
	// — the JSON cap must NOT apply or it would cripple file upload.
	var got string
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
	})

	mw := JSONBodyCap(10)(next)

	oversized := strings.Repeat("x", 50)
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(oversized))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=---abc")
	mw.ServeHTTP(httptest.NewRecorder(), req)

	if got != oversized {
		t.Fatalf("multipart body must NOT be capped (got %d bytes, want %d)", len(got), len(oversized))
	}
}

func TestJSONBodyCap_RespectsCharsetSuffix(t *testing.T) {
	// Real clients send "application/json; charset=utf-8" — the cap
	// must still apply, otherwise a polite Content-Type bypasses DoS
	// protection entirely.
	var capturedErr error
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, capturedErr = io.ReadAll(r.Body)
	})

	mw := JSONBodyCap(10)(next)

	oversized := strings.Repeat("x", 50)
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(oversized))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	mw.ServeHTTP(httptest.NewRecorder(), req)

	if capturedErr == nil {
		t.Fatal("expected MaxBytesError with charset-suffixed Content-Type, got nil")
	}
}

func TestMaxBodyBytes_CapsArbitraryContentType(t *testing.T) {
	// Outer absolute ceiling — applies regardless of Content-Type, so a
	// client spoofing "application/octet-stream" cannot stream past
	// the cap into json.NewDecoder via the bypass JSONBodyCap leaves
	// open by design.
	var capturedErr error
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, capturedErr = io.ReadAll(r.Body)
	})

	mw := MaxBodyBytes(10)(next)

	oversized := strings.Repeat("x", 50)
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(oversized))
	req.Header.Set("Content-Type", "application/octet-stream")
	mw.ServeHTTP(httptest.NewRecorder(), req)

	if capturedErr == nil {
		t.Fatal("expected MaxBytesError for octet-stream body, got nil")
	}
	var maxBytesErr *http.MaxBytesError
	if !errors.As(capturedErr, &maxBytesErr) {
		t.Fatalf("expected *http.MaxBytesError, got %T (%v)", capturedErr, capturedErr)
	}
}

func TestMaxBodyBytes_CapsNoContentTypeHeader(t *testing.T) {
	// Missing Content-Type must NOT bypass the outer cap — this is
	// exactly the bypass JSONBodyCap alone is vulnerable to.
	var capturedErr error
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, capturedErr = io.ReadAll(r.Body)
	})

	mw := MaxBodyBytes(10)(next)

	oversized := strings.Repeat("x", 50)
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(oversized))
	// Note: no Content-Type header set.
	mw.ServeHTTP(httptest.NewRecorder(), req)

	if capturedErr == nil {
		t.Fatal("expected MaxBytesError when Content-Type is absent, got nil")
	}
}

func TestMaxBodyBytes_StackedUnderJSONBodyCap(t *testing.T) {
	// Defence in depth: MaxBodyBytes is the outer (loose) ceiling,
	// JSONBodyCap layered inside trips first for JSON traffic because
	// MaxBytesReader honours the smallest cap in the chain.
	var capturedErr error
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, capturedErr = io.ReadAll(r.Body)
	})

	mw := MaxBodyBytes(1000)(JSONBodyCap(10)(next))

	oversized := strings.Repeat("x", 50)
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(oversized))
	req.Header.Set("Content-Type", "application/json")
	mw.ServeHTTP(httptest.NewRecorder(), req)

	if capturedErr == nil {
		t.Fatal("expected MaxBytesError from inner JSON cap, got nil")
	}
	var maxBytesErr *http.MaxBytesError
	if !errors.As(capturedErr, &maxBytesErr) {
		t.Fatalf("expected *http.MaxBytesError, got %T", capturedErr)
	}
	if maxBytesErr.Limit != 10 {
		t.Fatalf("expected inner JSON cap (10) to trip first, got Limit=%d", maxBytesErr.Limit)
	}
}

func TestMaxBodyBytesWithUploads_PathSelectsCap(t *testing.T) {
	// Small caps exercise the path-based selection without multi-MiB bodies:
	// general=10, upload=50.
	mw := MaxBodyBytesWithUploads(10, 50)

	readErr := func(path string, n int) error {
		var captured error
		next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			_, captured = io.ReadAll(r.Body)
		})
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(strings.Repeat("x", n)))
		mw(next).ServeHTTP(httptest.NewRecorder(), req)
		return captured
	}

	tests := []struct {
		name    string
		path    string
		n       int
		wantErr bool
	}{
		{"import under upload cap", "/api/prospects/import", 30, false},
		{"import over upload cap", "/api/leads/import", 60, true},
		{"non-import over general cap", "/api/prospects", 30, true},
		{"non-import under general cap", "/api/leads", 5, false},
		// Only an exact "/import" suffix loosens the cap. Variants fall back to
		// the general ceiling (and would 404 at the router anyway) — they must
		// never be a way around the tighter limit.
		{"trailing slash stays general", "/api/leads/import/", 30, true},
		{"uppercase stays general", "/api/leads/IMPORT", 30, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := readErr(tt.path, tt.n)
			if tt.wantErr {
				var maxErr *http.MaxBytesError
				if !errors.As(err, &maxErr) {
					t.Fatalf("expected *http.MaxBytesError, got %T (%v)", err, err)
				}
			} else if err != nil {
				t.Fatalf("expected read under cap to succeed, got %v", err)
			}
		})
	}
}
