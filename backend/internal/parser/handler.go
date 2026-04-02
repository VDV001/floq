package parser

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Handler struct{}

func RegisterRoutes(r chi.Router) {
	h := &Handler{}
	r.Post("/api/parser/website", h.scrapeWebsite())
}

func (h *Handler) scrapeWebsite() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			URL string `json:"url"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if body.URL == "" {
			writeError(w, http.StatusBadRequest, "url is required")
			return
		}

		emails, err := ScrapeEmails(body.URL)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"url":    body.URL,
			"emails": emails,
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
