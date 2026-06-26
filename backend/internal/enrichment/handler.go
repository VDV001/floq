package enrichment

import (
	"net/http"

	"github.com/daniil/floq/internal/enrichment/domain"
	"github.com/daniil/floq/internal/httputil"
	"github.com/go-chi/chi/v5"
)

// Handler serves the enrichment read API.
type Handler struct {
	uc *UseCase
}

// RegisterRoutes mounts the enrichment endpoints. The route is tenant-scoped:
// it returns only the calling user's enrichment for the domain.
func RegisterRoutes(r chi.Router, uc *UseCase) {
	h := &Handler{uc: uc}
	r.Get("/api/enrichment", h.getEnrichment())
}

// getEnrichment returns the enrichment for the company domain derived from the
// `email` query param. A free/invalid email or a missing row yields status
// "none" (HTTP 200) so the lead/prospect card renders cleanly.
func (h *Handler) getEnrichment() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		email := r.URL.Query().Get("email")
		if email == "" {
			httputil.WriteError(w, http.StatusBadRequest, "email query param is required")
			return
		}

		d, err := domain.NewDomain(email)
		if err != nil {
			// Free provider or malformed — nothing to enrich, not an error.
			httputil.WriteJSON(w, http.StatusOK, noneResponse(""))
			return
		}

		e, found, err := h.uc.Get(r.Context(), userID, d.String())
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to load enrichment")
			return
		}
		if !found {
			httputil.WriteJSON(w, http.StatusOK, noneResponse(d.String()))
			return
		}
		httputil.WriteJSON(w, http.StatusOK, toResponse(e))
	}
}
