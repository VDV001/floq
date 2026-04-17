package verify

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/prospects/domain"
	"github.com/go-chi/chi/v5"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

// ProspectRepository defines the prospect operations needed by the verify handler.
// ListProspects returns the read-model projection (Prospect + SourceName); the
// verify flow only reads the embedded Prospect fields, but we keep the
// signature aligned with the repository's canonical list interface.
type ProspectRepository interface {
	ListProspects(ctx context.Context, userID uuid.UUID) ([]domain.ProspectWithSource, error)
	GetProspect(ctx context.Context, id uuid.UUID) (*domain.Prospect, error)
	UpdateVerification(ctx context.Context, id uuid.UUID, status domain.VerifyStatus, score int, details string, verifiedAt time.Time) error
}

// Handler exposes verification endpoints.
type Handler struct {
	prospectRepo ProspectRepository
	bot          *tgbotapi.BotAPI // can be nil if not configured
}

// RegisterRoutes wires the verification endpoints into the given router.
func RegisterRoutes(r chi.Router, prospectRepo ProspectRepository, bot *tgbotapi.BotAPI) {
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
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Email == "" {
			httputil.WriteError(w, http.StatusBadRequest, "email is required")
			return
		}

		result := VerifyEmail(body.Email)
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

		prospectList, err := h.prospectRepo.ListProspects(r.Context(), userID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to list prospects")
			return
		}

		count := 0
		for _, p := range prospectList {
			if p.VerifyStatus != domain.VerifyStatusNotChecked {
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
				domain.VerifyStatus(emailResult.Status),
				emailResult.Score,
				string(detailsJSON),
				time.Now().UTC(),
			)
			if err != nil {
				continue
			}

			count++
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

		p, err := h.prospectRepo.GetProspect(r.Context(), id)
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
