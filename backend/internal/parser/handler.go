package parser

import (
	"encoding/json"
	"net/http"

	"github.com/daniil/floq/internal/httputil"
	"github.com/go-chi/chi/v5"
)

type Handler struct{}

func RegisterRoutes(r chi.Router) {
	h := &Handler{}
	r.Post("/api/parser/website", h.scrapeWebsite())
	r.Post("/api/parser/twogis", h.searchTwoGIS())
}

func (h *Handler) scrapeWebsite() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			URL string `json:"url"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if body.URL == "" {
			httputil.WriteError(w, http.StatusBadRequest, "url is required")
			return
		}

		emails, err := ScrapeEmails(body.URL)
		if err != nil {
			httputil.WriteError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}

		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"url":    body.URL,
			"emails": emails,
		})
	}
}

func (h *Handler) searchTwoGIS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query string `json:"query"`
			City  string `json:"city"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Query == "" {
			httputil.WriteError(w, http.StatusBadRequest, "query is required")
			return
		}
		if body.City == "" {
			body.City = "Москва"
		}

		results, err := Search2GIS(r.Context(), body.Query, body.City)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if results == nil {
			results = []TwoGISResult{}
		}
		httputil.WriteJSON(w, http.StatusOK, results)
	}
}
