package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupParserRouter() chi.Router {
	r := chi.NewRouter()
	RegisterRoutes(r, "test-api-key", nil)
	return r
}

// --- scrapeWebsite ---

func TestHandler_ScrapeWebsite_MissingURL(t *testing.T) {
	r := setupParserRouter()
	body := `{"url":""}`
	req := httptest.NewRequest("POST", "/api/parser/website", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ScrapeWebsite_InvalidJSON(t *testing.T) {
	r := setupParserRouter()
	req := httptest.NewRequest("POST", "/api/parser/website", bytes.NewBufferString("not json"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ScrapeWebsite_NoURLField(t *testing.T) {
	r := setupParserRouter()
	body := `{}`
	req := httptest.NewRequest("POST", "/api/parser/website", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- searchTwoGIS ---

func TestHandler_SearchTwoGIS_MissingQuery(t *testing.T) {
	r := setupParserRouter()
	body := `{"query":"","city":"Москва"}`
	req := httptest.NewRequest("POST", "/api/parser/twogis", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_SearchTwoGIS_InvalidJSON(t *testing.T) {
	r := setupParserRouter()
	req := httptest.NewRequest("POST", "/api/parser/twogis", bytes.NewBufferString("bad"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_SearchTwoGIS_DefaultCity(t *testing.T) {
	// We can't mock the external 2GIS API, but we can verify the handler
	// doesn't reject a request with empty city (defaults to Москва).
	// The actual HTTP call to 2GIS will fail with our test key.
	r := setupParserRouter()
	body := `{"query":"рестораны","city":""}`
	req := httptest.NewRequest("POST", "/api/parser/twogis", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Should be 500 (2GIS call fails with test key) or 200 - not 400.
	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

// --- ScrapeEmails with test server ---

func TestScrapeEmails_WithTestServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<html><body>Contact info@testcompany.com</body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	emails, err := ScrapeEmails(srv.URL, nil)
	require.NoError(t, err)
	assert.Contains(t, emails, "info@testcompany.com")
}

func TestScrapeEmails_WithContactPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<html><body>Main page</body></html>`))
		case "/contacts":
			w.Write([]byte(`<html><body><a href="mailto:sales@company.com">Email</a></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	emails, err := ScrapeEmails(srv.URL, nil)
	require.NoError(t, err)
	assert.Contains(t, emails, "sales@company.com")
}

func TestScrapeEmails_FilterJunk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>
			noreply@company.com
			info@company.com
			admin@company.com
		</body></html>`))
	}))
	defer srv.Close()

	emails, err := ScrapeEmails(srv.URL, nil)
	require.NoError(t, err)
	assert.Contains(t, emails, "info@company.com")
	assert.NotContains(t, emails, "noreply@company.com")
	assert.NotContains(t, emails, "admin@company.com")
}

func TestScrapeEmails_Deduplication(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>
			hello@test.com
			Hello@Test.Com
			HELLO@TEST.COM
		</body></html>`))
	}))
	defer srv.Close()

	emails, err := ScrapeEmails(srv.URL, nil)
	require.NoError(t, err)
	count := 0
	for _, e := range emails {
		if e == "hello@test.com" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestScrapeEmails_NoSchema(t *testing.T) {
	// URL without schema should get https:// prepended.
	// This will fail to connect but proves the normalization path.
	_, err := ScrapeEmails("nonexistent.invalid.test", nil)
	assert.Error(t, err)
}

func TestScrapeEmails_InvalidURL(t *testing.T) {
	_, err := ScrapeEmails("://bad", nil)
	assert.Error(t, err)
}

func TestScrapeEmails_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := ScrapeEmails(srv.URL, nil)
	assert.Error(t, err)
}

func TestScrapeEmails_EmptyPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Write([]byte(`<html><body>No emails here</body></html>`))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	emails, err := ScrapeEmails(srv.URL, nil)
	require.NoError(t, err)
	assert.Empty(t, emails)
}

func TestHandler_ScrapeWebsite_Success(t *testing.T) {
	// Set up a test server to serve as the target URL.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<html><body>email: found@example.com</body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	r := setupParserRouter()
	body, _ := json.Marshal(map[string]string{"url": srv.URL})
	req := httptest.NewRequest("POST", "/api/parser/website", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, srv.URL, resp["url"])
}

// --- extractEmails additional ---

func TestExtractEmails_MultipleMailtoLinks(t *testing.T) {
	html := `
		<a href="mailto:a@test.com">A</a>
		<a href="mailto:b@test.com">B</a>
		<a href="mailto:a@test.com">A again</a>
	`
	emails := extractEmails(html)
	assert.Contains(t, emails, "a@test.com")
	assert.Contains(t, emails, "b@test.com")
	// Should be deduplicated
	count := 0
	for _, e := range emails {
		if e == "a@test.com" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestIsJunkEmail_EmptyEmail(t *testing.T) {
	assert.False(t, isJunkEmail(""))
}

func TestIsJunkEmail_EdgeCaseTestingDept(t *testing.T) {
	// "testing-dept@" has local part "testing-dept", which starts with "test"
	// but the junk rule for "test@" requires HasPrefix on the whole email.
	// The rule checks: strings.HasPrefix(lower, "test@") which is false.
	assert.False(t, isJunkEmail("testing-dept@company.com"))
}

// --- searchTwoGIS with test server ---

func TestHandler_SearchTwoGIS_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"items": []any{
					map[string]any{"name": "Test Corp", "address_name": "ул. Мира, 1"},
				},
			},
		})
	}))
	defer srv.Close()

	r := chi.NewRouter()
	h := &Handler{twoGIS: &TwoGISClient{APIKey: "test", baseURL: srv.URL}}
	r.Post("/api/parser/twogis", h.searchTwoGIS())

	body := `{"query":"restaurants","city":"Москва"}`
	req := httptest.NewRequest("POST", "/api/parser/twogis", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []TwoGISResult
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	require.Len(t, resp, 1)
	assert.Equal(t, "Test Corp", resp[0].Name)
}

func TestHandler_SearchTwoGIS_EmptyResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"items": []any{},
			},
		})
	}))
	defer srv.Close()

	r := chi.NewRouter()
	h := &Handler{twoGIS: &TwoGISClient{APIKey: "test", baseURL: srv.URL}}
	r.Post("/api/parser/twogis", h.searchTwoGIS())

	body := `{"query":"nothing","city":"Москва"}`
	req := httptest.NewRequest("POST", "/api/parser/twogis", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp []TwoGISResult
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Empty(t, resp)
}

func TestHandler_SearchTwoGIS_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	r := chi.NewRouter()
	h := &Handler{twoGIS: &TwoGISClient{APIKey: "test", baseURL: srv.URL}}
	r.Post("/api/parser/twogis", h.searchTwoGIS())

	body := `{"query":"test","city":"Москва"}`
	req := httptest.NewRequest("POST", "/api/parser/twogis", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_ScrapeWebsite_ScrapeError(t *testing.T) {
	// Target that returns non-200 → ScrapeEmails returns error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	r := setupParserRouter()
	body, _ := json.Marshal(map[string]string{"url": srv.URL})
	req := httptest.NewRequest("POST", "/api/parser/website", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

// --- ScrapeEmails additional ---

func TestScrapeEmails_Over50Emails(t *testing.T) {
	// Page with > 50 unique emails → should be capped at 50
	var body string
	for i := 0; i < 60; i++ {
		body += fmt.Sprintf("user%d@company%d.com ", i, i)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Write([]byte("<html><body>" + body + "</body></html>"))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	emails, err := ScrapeEmails(srv.URL, nil)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(emails), 50)
}

func TestScrapeEmails_TooManyRedirects(t *testing.T) {
	redirectCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectCount++
		http.Redirect(w, r, r.URL.String()+"?r="+fmt.Sprintf("%d", redirectCount), http.StatusFound)
	}))
	defer srv.Close()

	_, err := ScrapeEmails(srv.URL, nil)
	assert.Error(t, err)
}

// --- TwoGISResult marshalling ---

func TestTwoGISResult_JSONMarshalling(t *testing.T) {
	r := TwoGISResult{
		Name:     "ООО Ромашка",
		Address:  "ул. Ленина, 1",
		Phone:    "+79001234567",
		Category: "Рестораны",
		Website:  "https://romashka.ru",
		City:     "Москва",
	}

	data, err := json.Marshal(r)
	require.NoError(t, err)

	var decoded TwoGISResult
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, r, decoded)
}
