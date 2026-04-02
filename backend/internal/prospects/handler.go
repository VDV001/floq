package prospects

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	uc *UseCase
}

func getUserID(r *http.Request) uuid.UUID {
	uid, _ := r.Context().Value("user_id").(uuid.UUID)
	return uid
}

func RegisterRoutes(r chi.Router, uc *UseCase) {
	h := &Handler{uc: uc}
	r.Get("/api/prospects", h.listProspects())
	r.Post("/api/prospects", h.createProspect())
	r.Post("/api/prospects/import", h.importCSV())
	r.Get("/api/prospects/{id}", h.getProspect())
	r.Delete("/api/prospects/{id}", h.deleteProspect())
}

func parseIDParam(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "id"))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (h *Handler) listProspects() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		prospects, err := h.uc.ListProspects(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list prospects")
			return
		}
		if prospects == nil {
			prospects = []Prospect{}
		}
		writeJSON(w, http.StatusOK, prospects)
	}
}

func (h *Handler) createProspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name    string `json:"name"`
			Company string `json:"company"`
			Title   string `json:"title"`
			Email   string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}

		now := time.Now().UTC()
		p := &Prospect{
			ID:        uuid.New(),
			UserID:    getUserID(r),
			Name:      body.Name,
			Company:   body.Company,
			Title:     body.Title,
			Email:     body.Email,
			Source:    "manual",
			Status:    "new",
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := h.uc.CreateProspect(r.Context(), p); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create prospect")
			return
		}
		writeJSON(w, http.StatusCreated, p)
	}
}

func (h *Handler) importCSV() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "missing file field")
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read file")
			return
		}

		userID := getUserID(r)
		count, err := h.uc.ImportCSV(r.Context(), userID, data)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]int{"imported": count})
	}
}

func (h *Handler) getProspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid prospect id")
			return
		}
		prospect, err := h.uc.GetProspect(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get prospect")
			return
		}
		if prospect == nil {
			writeError(w, http.StatusNotFound, "prospect not found")
			return
		}
		writeJSON(w, http.StatusOK, prospect)
	}
}

func (h *Handler) deleteProspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid prospect id")
			return
		}
		if err := h.uc.DeleteProspect(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete prospect")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
