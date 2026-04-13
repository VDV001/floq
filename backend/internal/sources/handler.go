package sources

import (
	"encoding/json"
	"net/http"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/sources/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	uc *UseCase
}

func RegisterRoutes(r chi.Router, uc *UseCase) {
	h := &Handler{uc: uc}
	r.Get("/api/sources", h.listCategories())
	r.Post("/api/sources/categories", h.createCategory())
	r.Put("/api/sources/categories/{id}", h.updateCategory())
	r.Delete("/api/sources/categories/{id}", h.deleteCategory())
	r.Post("/api/sources", h.createSource())
	r.Put("/api/sources/{id}", h.updateSource())
	r.Delete("/api/sources/{id}", h.deleteSource())
}

func (h *Handler) listCategories() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		cats, err := h.uc.ListCategories(r.Context(), userID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to list sources")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, CategoriesToResponse(cats))
	}
}

func (h *Handler) createCategory() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		cat, err := h.uc.CreateCategory(r.Context(), userID, body.Name)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		cws := domain.CategoryWithSources{Category: *cat, Sources: []domain.Source{}}
		httputil.WriteJSON(w, http.StatusCreated, CategoryWithSourcesToResponse(&cws))
	}
}

func (h *Handler) updateCategory() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid category id")
			return
		}

		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := h.uc.UpdateCategory(r.Context(), id, body.Name); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) deleteCategory() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid category id")
			return
		}
		if err := h.uc.DeleteCategory(r.Context(), id); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to delete category")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) createSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var body struct {
			CategoryID uuid.UUID `json:"category_id"`
			Name       string    `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		src, err := h.uc.CreateSource(r.Context(), userID, body.CategoryID, body.Name)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.WriteJSON(w, http.StatusCreated, SourceToResponse(src))
	}
}

func (h *Handler) updateSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid source id")
			return
		}

		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := h.uc.UpdateSource(r.Context(), id, body.Name); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) deleteSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid source id")
			return
		}
		if err := h.uc.DeleteSource(r.Context(), id); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to delete source")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
