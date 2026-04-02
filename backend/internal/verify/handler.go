package verify

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/daniil/floq/internal/prospects"
	"github.com/go-chi/chi/v5"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

// Handler exposes verification endpoints.
type Handler struct {
	prospectRepo *prospects.Repository
	bot          *tgbotapi.BotAPI // can be nil if not configured
}

// RegisterRoutes wires the verification endpoints into the given router.
func RegisterRoutes(r chi.Router, prospectRepo *prospects.Repository, bot *tgbotapi.BotAPI) {
	h := &Handler{prospectRepo: prospectRepo, bot: bot}
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
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Email == "" {
			writeError(w, http.StatusBadRequest, "email is required")
			return
		}

		result := VerifyEmail(body.Email)
		writeJSON(w, http.StatusOK, result)
	}
}

func (h *Handler) verifyBatch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value("user_id").(uuid.UUID)
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing user_id in context")
			return
		}

		prospectList, err := h.prospectRepo.ListProspects(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list prospects")
			return
		}

		count := 0
		for _, p := range prospectList {
			if p.VerifyStatus != "not_checked" {
				continue
			}

			emailResult := VerifyEmail(p.Email)

			details := map[string]any{
				"email": emailResult,
			}

			if p.TelegramUsername != "" && h.bot != nil {
				tgResult := VerifyTelegram(h.bot, p.TelegramUsername)
				details["telegram"] = tgResult
			}

			detailsJSON, err := json.Marshal(details)
			if err != nil {
				continue
			}

			err = h.prospectRepo.UpdateVerification(
				r.Context(),
				p.ID,
				emailResult.Status,
				emailResult.Score,
				string(detailsJSON),
				time.Now().UTC(),
			)
			if err != nil {
				continue
			}

			count++
		}

		writeJSON(w, http.StatusOK, map[string]int{"verified": count})
	}
}

func (h *Handler) getVerifyStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid prospect id")
			return
		}

		p, err := h.prospectRepo.GetProspect(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get prospect")
			return
		}
		if p == nil {
			writeError(w, http.StatusNotFound, "prospect not found")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"verify_status":  p.VerifyStatus,
			"verify_score":   p.VerifyScore,
			"verify_details": p.VerifyDetails,
			"verified_at":    p.VerifiedAt,
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
