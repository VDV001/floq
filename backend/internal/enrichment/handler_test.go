package enrichment_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daniil/floq/internal/enrichment"
	"github.com/daniil/floq/internal/enrichment/domain"
	"github.com/daniil/floq/internal/httputil"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mountEnrichment(uc *enrichment.UseCase) chi.Router {
	r := chi.NewRouter()
	enrichment.RegisterRoutes(r, uc)
	return r
}

func getReq(t *testing.T, r chi.Router, target string, userID *uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if userID != nil {
		req = req.WithContext(httputil.WithUserID(req.Context(), *userID))
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestHandler_Get_Enriched(t *testing.T) {
	userID := uuid.New()
	d, _ := domain.NewDomain("ivan@acme.ru")
	e, _ := domain.NewPendingEnrichment(userID, d)
	e.MarkEnriched(domain.CompanyProfile{Title: "Acme", Emails: []string{"info@acme.ru"}}, 3600)
	uc := newUC(&fakeStore{getResult: e}, fakeFetcher{}, fakeExtractor{}, fakeLimiter{allow: true})

	rec := getReq(t, mountEnrichment(uc), "/api/enrichment?email=ivan@acme.ru", &userID)
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Domain  string `json:"domain"`
		Status  string `json:"status"`
		Profile struct {
			Title  string   `json:"title"`
			Emails []string `json:"emails"`
		} `json:"profile"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "acme.ru", body.Domain)
	assert.Equal(t, "enriched", body.Status)
	assert.Equal(t, "Acme", body.Profile.Title)
	assert.Equal(t, []string{"info@acme.ru"}, body.Profile.Emails)
}

func TestHandler_Get_ExposesPhase2Fields(t *testing.T) {
	userID := uuid.New()
	d, _ := domain.NewDomain("ivan@acme.ru")
	e, _ := domain.NewPendingEnrichment(userID, d)
	e.MarkEnriched(domain.CompanyProfile{
		Title:       "Acme",
		Industry:    "fintech",
		CompanySize: domain.CompanySizeMedium,
	}, 3600)
	uc := newUC(&fakeStore{getResult: e}, fakeFetcher{}, fakeExtractor{}, fakeLimiter{allow: true})

	rec := getReq(t, mountEnrichment(uc), "/api/enrichment?email=ivan@acme.ru", &userID)
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Profile struct {
			Industry    string `json:"industry"`
			CompanySize string `json:"companySize"`
		} `json:"profile"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "fintech", body.Profile.Industry)
	assert.Equal(t, "medium", body.Profile.CompanySize)
}

func TestHandler_Get_ExposesLegalDetails(t *testing.T) {
	userID := uuid.New()
	d, _ := domain.NewDomain("ivan@acme.ru")
	e, _ := domain.NewPendingEnrichment(userID, d)
	e.MarkEnriched(domain.CompanyProfile{
		Title: "Acme",
		Legal: domain.LegalDetails{INN: "7707083893", OGRN: "1027700132195", Address: "Москва", OKVED: "62.01", Status: "ACTIVE", FullName: "ООО Акме"},
	}, 3600)
	uc := newUC(&fakeStore{getResult: e}, fakeFetcher{}, fakeExtractor{}, fakeLimiter{allow: true})

	rec := getReq(t, mountEnrichment(uc), "/api/enrichment?email=ivan@acme.ru", &userID)
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Profile struct {
			Legal struct {
				INN     string `json:"inn"`
				OGRN    string `json:"ogrn"`
				Address string `json:"address"`
				OKVED   string `json:"okved"`
				Status  string `json:"status"`
			} `json:"legal"`
		} `json:"profile"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "7707083893", body.Profile.Legal.INN)
	assert.Equal(t, "1027700132195", body.Profile.Legal.OGRN)
	assert.Equal(t, "Москва", body.Profile.Legal.Address)
	assert.Equal(t, "62.01", body.Profile.Legal.OKVED)
	assert.Equal(t, "ACTIVE", body.Profile.Legal.Status)
}

func TestHandler_Get_NotFoundReturnsNone(t *testing.T) {
	userID := uuid.New()
	uc := newUC(&fakeStore{getResult: nil}, fakeFetcher{}, fakeExtractor{}, fakeLimiter{allow: true})

	rec := getReq(t, mountEnrichment(uc), "/api/enrichment?email=ivan@acme.ru", &userID)
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "none", body.Status, "no row yet → status none, not an error")
}

func TestHandler_Get_FreeEmailReturnsNone(t *testing.T) {
	userID := uuid.New()
	uc := newUC(&fakeStore{}, fakeFetcher{}, fakeExtractor{}, fakeLimiter{allow: true})

	rec := getReq(t, mountEnrichment(uc), "/api/enrichment?email=ivan@gmail.com", &userID)
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "none", body.Status)
}

func TestHandler_Get_Unauthorized(t *testing.T) {
	uc := newUC(&fakeStore{}, fakeFetcher{}, fakeExtractor{}, fakeLimiter{allow: true})
	rec := getReq(t, mountEnrichment(uc), "/api/enrichment?email=ivan@acme.ru", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_Get_MissingEmail(t *testing.T) {
	userID := uuid.New()
	uc := newUC(&fakeStore{}, fakeFetcher{}, fakeExtractor{}, fakeLimiter{allow: true})
	rec := getReq(t, mountEnrichment(uc), "/api/enrichment", &userID)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
