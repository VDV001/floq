package prospects

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/prospects/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRouter(uc *UseCase) *chi.Mux {
	r := chi.NewRouter()
	RegisterRoutes(r, uc)
	return r
}

func authedRequest(r *http.Request, userID uuid.UUID) *http.Request {
	ctx := httputil.WithUserID(r.Context(), userID)
	return r.WithContext(ctx)
}

// --- listProspects ---

func TestHandler_ListProspects_OK(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	id := uuid.New()
	repo.prospects[id] = &domain.Prospect{
		ID: id, UserID: userID, Name: "Alice",
		Status: domain.ProspectStatusNew, VerifyStatus: domain.VerifyStatusNotChecked,
	}
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/api/prospects", nil)
	req = authedRequest(req, userID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []ProspectResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "Alice", resp[0].Name)
}

func TestHandler_ListProspects_Unauthorized(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/api/prospects", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_ListProspects_Empty(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/api/prospects", nil)
	req = authedRequest(req, uuid.New())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []ProspectResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp)
}

// --- createProspect ---

func TestHandler_CreateProspect_OK(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	router := setupRouter(uc)
	userID := uuid.New()

	body := map[string]string{
		"name":    "Alice",
		"company": "Acme",
		"title":   "CEO",
		"email":   "alice@acme.com",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/prospects", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = authedRequest(req, userID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp ProspectResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Alice", resp.Name)
	assert.Equal(t, "manual", resp.Source)
}

func TestHandler_CreateProspect_MissingName(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	body := map[string]string{"company": "Acme"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/prospects", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = authedRequest(req, uuid.New())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_CreateProspect_InvalidBody(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	req := httptest.NewRequest(http.MethodPost, "/api/prospects", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req = authedRequest(req, uuid.New())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_CreateProspect_Unauthorized(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	body := map[string]string{"name": "Alice"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/prospects", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- getProspect ---

func TestHandler_GetProspect_OK(t *testing.T) {
	repo := newMockRepo()
	id := uuid.New()
	repo.prospects[id] = &domain.Prospect{
		ID: id, Name: "Alice",
		Status: domain.ProspectStatusNew, VerifyStatus: domain.VerifyStatusNotChecked,
	}
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/api/prospects/"+id.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp ProspectResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Alice", resp.Name)
}

func TestHandler_GetProspect_NotFound(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/api/prospects/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_GetProspect_InvalidID(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/api/prospects/not-a-uuid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- deleteProspect ---

func TestHandler_DeleteProspect_OK(t *testing.T) {
	repo := newMockRepo()
	id := uuid.New()
	repo.prospects[id] = &domain.Prospect{ID: id, Name: "Alice"}
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	req := httptest.NewRequest(http.MethodDelete, "/api/prospects/"+id.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.NotContains(t, repo.prospects, id)
}

func TestHandler_DeleteProspect_InvalidID(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	req := httptest.NewRequest(http.MethodDelete, "/api/prospects/bad-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- exportCSV ---

func TestHandler_ExportCSV_OK(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	id := uuid.New()
	repo.prospects[id] = &domain.Prospect{
		ID: id, UserID: userID, Name: "Alice", Company: "Acme", Title: "CEO", Email: "alice@acme.com",
		Source: "manual", Status: domain.ProspectStatusNew, VerifyStatus: domain.VerifyStatusNotChecked,
	}
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/api/prospects/export", nil)
	req = authedRequest(req, userID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, w.Body.String(), "Alice,Acme,CEO,alice@acme.com")
}

func TestHandler_ExportCSV_Unauthorized(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/api/prospects/export", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- importCSV ---

func TestHandler_ImportCSV_OK(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	router := setupRouter(uc)
	userID := uuid.New()

	csvData := "name,company,title,email\nAlice,Acme,CEO,alice@acme.com\n"
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "prospects.csv")
	require.NoError(t, err)
	_, err = fw.Write([]byte(csvData))
	require.NoError(t, err)
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/prospects/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req = authedRequest(req, userID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp ImportReportResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Imported)
	assert.Empty(t, resp.Skipped)
}

func TestHandler_ImportCSV_TooLarge(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	// Tiny caps exercise the 413 path without a real 50 MiB body: upload=50 bytes.
	r := chi.NewRouter()
	r.Use(httputil.MaxBodyBytesWithUploads(10, 50))
	RegisterRoutes(r, uc)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "big.csv")
	require.NoError(t, err)
	_, err = fw.Write(bytes.Repeat([]byte("x"), 200)) // exceeds the 50-byte upload cap
	require.NoError(t, err)
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/prospects/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req = authedRequest(req, uuid.New())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code, "oversized upload must be 413, not 400")
}

func TestHandler_ImportCSV_MissingFile(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	req := httptest.NewRequest(http.MethodPost, "/api/prospects/import", nil)
	req = authedRequest(req, uuid.New())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_ImportCSV_Unauthorized(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	csvData := "name,company,title,email\nAlice,Acme,CEO,alice@acme.com\n"
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "prospects.csv")
	require.NoError(t, err)
	_, err = fw.Write([]byte(csvData))
	require.NoError(t, err)
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/prospects/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	// no auth context
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_ListProspects_Error(t *testing.T) {
	repo := &mockErrorRepo{listErr: fmt.Errorf("db down")}
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/api/prospects", nil)
	req = authedRequest(req, uuid.New())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_CreateProspect_UseCaseError(t *testing.T) {
	repo := &mockErrorRepo{createErr: fmt.Errorf("db down")}
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	body := map[string]string{"name": "Alice"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/prospects", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = authedRequest(req, uuid.New())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GetProspect_Error(t *testing.T) {
	repo := &mockErrorRepo{getErr: fmt.Errorf("db down")}
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/api/prospects/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_DeleteProspect_Error(t *testing.T) {
	repo := &mockErrorRepo{deleteErr: fmt.Errorf("db down")}
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	req := httptest.NewRequest(http.MethodDelete, "/api/prospects/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_ExportCSV_Error(t *testing.T) {
	repo := &mockErrorRepo{listErr: fmt.Errorf("db down")}
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/api/prospects/export", nil)
	req = authedRequest(req, uuid.New())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_ImportCSV_BadCSV(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))
	router := setupRouter(uc)

	csvData := "wrong,header\ndata\n"
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "bad.csv")
	require.NoError(t, err)
	_, err = fw.Write([]byte(csvData))
	require.NoError(t, err)
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/prospects/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req = authedRequest(req, uuid.New())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_SetConsent(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	p, err := domain.NewProspect(userID, "Bob", "Acme", "CEO", "bob@acme.com", "manual")
	require.NoError(t, err)
	repo.prospects[p.ID] = p
	uc := NewUseCase(repo)
	router := setupRouter(uc)

	do := func(id, body string, uid *uuid.UUID) int {
		req := httptest.NewRequest(http.MethodPost, "/api/prospects/"+id+"/consent", bytes.NewBufferString(body))
		if uid != nil {
			req = authedRequest(req, *uid)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}

	assert.Equal(t, http.StatusOK, do(p.ID.String(), `{"status":"obtained"}`, &userID))
	assert.Equal(t, domain.ConsentStatusObtained, repo.prospects[p.ID].Consent.Status)
	assert.Equal(t, http.StatusBadRequest, do(p.ID.String(), `{"status":"bogus"}`, &userID))
	assert.Equal(t, http.StatusUnauthorized, do(p.ID.String(), `{"status":"obtained"}`, nil))
	assert.Equal(t, http.StatusNotFound, do(uuid.New().String(), `{"status":"obtained"}`, &userID))
}
