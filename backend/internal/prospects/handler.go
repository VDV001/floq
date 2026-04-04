package prospects

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/daniil/floq/internal/httputil"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	uc *UseCase
}

func RegisterRoutes(r chi.Router, uc *UseCase) {
	h := &Handler{uc: uc}
	r.Get("/api/prospects", h.listProspects())
	r.Post("/api/prospects", h.createProspect())
	r.Post("/api/prospects/import", h.importCSV())
	r.Get("/api/prospects/{id}", h.getProspect())
	r.Delete("/api/prospects/{id}", h.deleteProspect())
}

func (h *Handler) listProspects() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		prospects, err := h.uc.ListProspects(r.Context(), userID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to list prospects")
			return
		}
		if prospects == nil {
			prospects = []Prospect{}
		}
		httputil.WriteJSON(w, http.StatusOK, prospects)
	}
}

func (h *Handler) createProspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var body struct {
			Name             string `json:"name"`
			Company          string `json:"company"`
			Title            string `json:"title"`
			Email            string `json:"email"`
			Phone            string `json:"phone"`
			TelegramUsername string `json:"telegram_username"`
			Industry         string `json:"industry"`
			CompanySize      string `json:"company_size"`
			Context          string `json:"context"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Name == "" {
			httputil.WriteError(w, http.StatusBadRequest, "name is required")
			return
		}

		if body.Email != "" {
			existing, err := h.uc.FindByEmail(r.Context(), userID, body.Email)
			if err != nil {
				httputil.WriteError(w, http.StatusInternalServerError, "dedup check failed")
				return
			}
			if existing != nil {
				httputil.WriteError(w, http.StatusConflict, "проспект с таким email уже существует")
				return
			}
		}

		now := time.Now().UTC()
		p := &Prospect{
			ID:               uuid.New(),
			UserID:           userID,
			Name:             body.Name,
			Company:          body.Company,
			Title:            body.Title,
			Email:            body.Email,
			Phone:            body.Phone,
			TelegramUsername: body.TelegramUsername,
			Industry:         body.Industry,
			CompanySize:      body.CompanySize,
			Context:          body.Context,
			Source:           "manual",
			Status:           "new",
			VerifyStatus:     "not_checked",
			VerifyDetails:    "{}",
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		if err := h.uc.CreateProspect(r.Context(), p); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to create prospect")
			return
		}
		httputil.WriteJSON(w, http.StatusCreated, p)
	}
}

func (h *Handler) importCSV() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("file")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "missing file field")
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "failed to read file")
			return
		}

		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		count, err := h.uc.ImportCSV(r.Context(), userID, data)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		httputil.WriteJSON(w, http.StatusOK, map[string]int{"imported": count})
	}
}

func (h *Handler) getProspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid prospect id")
			return
		}
		prospect, err := h.uc.GetProspect(r.Context(), id)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to get prospect")
			return
		}
		if prospect == nil {
			httputil.WriteError(w, http.StatusNotFound, "prospect not found")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, prospect)
	}
}

func (h *Handler) deleteProspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid prospect id")
			return
		}
		if err := h.uc.DeleteProspect(r.Context(), id); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to delete prospect")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
