package verify

import (
	"encoding/json"
	"net/http"

	"github.com/daniil/floq/internal/httputil"
	"github.com/go-chi/chi/v5"
)

// Handler exposes verification HTTP endpoints. It is intentionally thin:
// parse → delegate to UseCase → write response.
type Handler struct {
	uc *UseCase
}

// RegisterRoutes wires the verification endpoints into the given router.
func RegisterRoutes(r chi.Router, uc *UseCase) {
	h := &Handler{uc: uc}
	r.Post("/api/verify/email", h.verifyEmailSingle())
	r.Post("/api/verify/batch", h.verifyBatch())
	r.Get("/api/prospects/{id}/verify", h.getVerifyStatus())
}

func (h *Handler) verifyEmailSingle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Email == "" {
			httputil.WriteError(w, http.StatusBadRequest, "email is required")
			return
		}

		result := h.uc.VerifyEmailSingle(r.Context(), body.Email)
		httputil.WriteJSON(w, http.StatusOK, result)
	}
}

func (h *Handler) verifyBatch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		count, err := h.uc.VerifyBatch(r.Context(), userID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to list prospects")
			return
		}

		httputil.WriteJSON(w, http.StatusOK, map[string]int{"verified": count})
	}
}

func (h *Handler) getVerifyStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid prospect id")
			return
		}

		p, err := h.uc.GetVerifyStatus(r.Context(), id)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to get prospect")
			return
		}
		if p == nil {
			httputil.WriteError(w, http.StatusNotFound, "prospect not found")
			return
		}

		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"verify_status":  p.VerifyStatus,
			"verify_score":   p.VerifyScore,
			"verify_details": p.VerifyDetails,
			"verified_at":    p.VerifiedAt,
		})
	}
}
