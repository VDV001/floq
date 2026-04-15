package sources

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/sources/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errorMockRepo always returns an error for operations that can fail.
type errorMockRepo struct {
	mockRepo
	err error
}

func (m *errorMockRepo) EnsureDefaults(_ context.Context, _ uuid.UUID) error { return m.err }
func (m *errorMockRepo) ListCategories(_ context.Context, _ uuid.UUID) ([]domain.CategoryWithSources, error) {
	return nil, m.err
}

// errorDeleteRepo returns errors from Delete operations.
type errorDeleteRepo struct {
	mockRepo
	err error
}

func (m *errorDeleteRepo) DeleteCategory(_ context.Context, _ uuid.UUID) error { return m.err }
func (m *errorDeleteRepo) DeleteSource(_ context.Context, _ uuid.UUID) error   { return m.err }
func (m *errorDeleteRepo) EnsureDefaults(_ context.Context, _ uuid.UUID) error { return nil }

func setupRouter() (chi.Router, *mockRepo) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithStatsReader(repo))
	r := chi.NewRouter()
	RegisterRoutes(r, uc)
	return r, repo
}

func withUser(r *http.Request, userID uuid.UUID) *http.Request {
	ctx := httputil.WithUserID(r.Context(), userID)
	return r.WithContext(ctx)
}

func TestHandler_ListCategories_Unauthorized(t *testing.T) {
	r, _ := setupRouter()
	req := httptest.NewRequest("GET", "/api/sources", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_ListCategories_Success(t *testing.T) {
	r, _ := setupRouter()
	userID := uuid.New()
	req := withUser(httptest.NewRequest("GET", "/api/sources", nil), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
}

func TestHandler_CreateCategory_Success(t *testing.T) {
	r, _ := setupRouter()
	userID := uuid.New()
	body := `{"name":"Парсинг"}`
	req := withUser(httptest.NewRequest("POST", "/api/sources/categories", bytes.NewBufferString(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp CategoryResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "Парсинг", resp.Name)
	assert.NotEqual(t, uuid.Nil, resp.ID)
}

func TestHandler_CreateCategory_Unauthorized(t *testing.T) {
	r, _ := setupRouter()
	body := `{"name":"Test"}`
	req := httptest.NewRequest("POST", "/api/sources/categories", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_CreateCategory_EmptyName(t *testing.T) {
	r, _ := setupRouter()
	userID := uuid.New()
	body := `{"name":""}`
	req := withUser(httptest.NewRequest("POST", "/api/sources/categories", bytes.NewBufferString(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_CreateCategory_InvalidJSON(t *testing.T) {
	r, _ := setupRouter()
	userID := uuid.New()
	req := withUser(httptest.NewRequest("POST", "/api/sources/categories", bytes.NewBufferString("not json")), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_UpdateCategory_Success(t *testing.T) {
	r, repo := setupRouter()
	userID := uuid.New()

	// Create a category first
	uc := NewUseCase(repo)
	cat, _ := uc.CreateCategory(context.Background(), userID, "Old")

	body := `{"name":"New"}`
	req := httptest.NewRequest("PUT", "/api/sources/categories/"+cat.ID.String(), bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestHandler_UpdateCategory_InvalidID(t *testing.T) {
	r, _ := setupRouter()
	body := `{"name":"New"}`
	req := httptest.NewRequest("PUT", "/api/sources/categories/not-a-uuid", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_DeleteCategory_Success(t *testing.T) {
	r, repo := setupRouter()
	userID := uuid.New()

	uc := NewUseCase(repo)
	cat, _ := uc.CreateCategory(context.Background(), userID, "ToDelete")

	req := httptest.NewRequest("DELETE", "/api/sources/categories/"+cat.ID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestHandler_DeleteCategory_InvalidID(t *testing.T) {
	r, _ := setupRouter()
	req := httptest.NewRequest("DELETE", "/api/sources/categories/bad-id", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_CreateSource_Success(t *testing.T) {
	r, repo := setupRouter()
	userID := uuid.New()

	uc := NewUseCase(repo)
	cat, _ := uc.CreateCategory(context.Background(), userID, "Cat")

	body, _ := json.Marshal(map[string]any{
		"category_id": cat.ID,
		"name":        "CSV файл",
	})
	req := withUser(httptest.NewRequest("POST", "/api/sources", bytes.NewBuffer(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp SourceResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "CSV файл", resp.Name)
}

func TestHandler_CreateSource_Unauthorized(t *testing.T) {
	r, _ := setupRouter()
	body := `{"category_id":"` + uuid.New().String() + `","name":"Test"}`
	req := httptest.NewRequest("POST", "/api/sources", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_CreateSource_EmptyName(t *testing.T) {
	r, _ := setupRouter()
	userID := uuid.New()
	body := `{"category_id":"` + uuid.New().String() + `","name":""}`
	req := withUser(httptest.NewRequest("POST", "/api/sources", bytes.NewBufferString(body)), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_UpdateSource_Success(t *testing.T) {
	r, repo := setupRouter()
	userID := uuid.New()

	uc := NewUseCase(repo)
	cat, _ := uc.CreateCategory(context.Background(), userID, "Cat")
	src, _ := uc.CreateSource(context.Background(), userID, cat.ID, "Old")

	body := `{"name":"New"}`
	req := httptest.NewRequest("PUT", "/api/sources/"+src.ID.String(), bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestHandler_UpdateSource_InvalidID(t *testing.T) {
	r, _ := setupRouter()
	body := `{"name":"New"}`
	req := httptest.NewRequest("PUT", "/api/sources/bad-uuid", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_DeleteSource_Success(t *testing.T) {
	r, repo := setupRouter()
	userID := uuid.New()

	uc := NewUseCase(repo)
	cat, _ := uc.CreateCategory(context.Background(), userID, "Cat")
	src, _ := uc.CreateSource(context.Background(), userID, cat.ID, "Src")

	req := httptest.NewRequest("DELETE", "/api/sources/"+src.ID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestHandler_DeleteSource_InvalidID(t *testing.T) {
	r, _ := setupRouter()
	req := httptest.NewRequest("DELETE", "/api/sources/bad", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_UpdateCategory_InvalidJSON(t *testing.T) {
	r, repo := setupRouter()
	userID := uuid.New()
	uc := NewUseCase(repo)
	cat, _ := uc.CreateCategory(context.Background(), userID, "Cat")

	req := httptest.NewRequest("PUT", "/api/sources/categories/"+cat.ID.String(), bytes.NewBufferString("not json"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_UpdateCategory_EmptyName(t *testing.T) {
	r, repo := setupRouter()
	userID := uuid.New()
	uc := NewUseCase(repo)
	cat, _ := uc.CreateCategory(context.Background(), userID, "Cat")

	body := `{"name":""}`
	req := httptest.NewRequest("PUT", "/api/sources/categories/"+cat.ID.String(), bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_UpdateSource_InvalidJSON(t *testing.T) {
	r, repo := setupRouter()
	userID := uuid.New()
	uc := NewUseCase(repo)
	cat, _ := uc.CreateCategory(context.Background(), userID, "Cat")
	src, _ := uc.CreateSource(context.Background(), userID, cat.ID, "Src")

	req := httptest.NewRequest("PUT", "/api/sources/"+src.ID.String(), bytes.NewBufferString("bad"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_UpdateSource_EmptyName(t *testing.T) {
	r, repo := setupRouter()
	userID := uuid.New()
	uc := NewUseCase(repo)
	cat, _ := uc.CreateCategory(context.Background(), userID, "Cat")
	src, _ := uc.CreateSource(context.Background(), userID, cat.ID, "Src")

	body := `{"name":""}`
	req := httptest.NewRequest("PUT", "/api/sources/"+src.ID.String(), bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_CreateSource_InvalidJSON(t *testing.T) {
	r, _ := setupRouter()
	userID := uuid.New()
	req := withUser(httptest.NewRequest("POST", "/api/sources", bytes.NewBufferString("bad")), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ListCategories_Error(t *testing.T) {
	repo := &errorMockRepo{err: assert.AnError}
	uc := NewUseCase(repo, WithStatsReader(&mockStatsReader{}))
	r := chi.NewRouter()
	RegisterRoutes(r, uc)

	userID := uuid.New()
	req := withUser(httptest.NewRequest("GET", "/api/sources", nil), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_Stats_Error(t *testing.T) {
	repo := newMockRepo()
	sr := &mockStatsReader{err: assert.AnError}
	uc := NewUseCase(repo, WithStatsReader(sr))
	r := chi.NewRouter()
	RegisterRoutes(r, uc)

	userID := uuid.New()
	req := withUser(httptest.NewRequest("GET", "/api/sources/stats", nil), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_DeleteCategory_Error(t *testing.T) {
	repo := &errorDeleteRepo{err: assert.AnError}
	uc := NewUseCase(repo)
	r := chi.NewRouter()
	RegisterRoutes(r, uc)

	id := uuid.New()
	req := httptest.NewRequest("DELETE", "/api/sources/categories/"+id.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_DeleteSource_Error(t *testing.T) {
	repo := &errorDeleteRepo{err: assert.AnError}
	uc := NewUseCase(repo)
	r := chi.NewRouter()
	RegisterRoutes(r, uc)

	id := uuid.New()
	req := httptest.NewRequest("DELETE", "/api/sources/"+id.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_Stats_Unauthorized(t *testing.T) {
	r, _ := setupRouter()
	req := httptest.NewRequest("GET", "/api/sources/stats", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_Stats_Success(t *testing.T) {
	r, _ := setupRouter()
	userID := uuid.New()
	req := withUser(httptest.NewRequest("GET", "/api/sources/stats", nil), userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
