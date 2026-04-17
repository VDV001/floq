package leads

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
)

// prospectSuggestionResponse mirrors domain.ProspectSuggestion for JSON.
type prospectSuggestionResponse struct {
	ProspectID       string `json:"prospect_id"`
	Name             string `json:"name"`
	Company          string `json:"company"`
	Email            string `json:"email"`
	TelegramUsername string `json:"telegram_username"`
	SourceName       string `json:"source_name"`
	Status           string `json:"status"`
	Confidence       string `json:"confidence"`
}

func suggestionsToResponse(suggestions []domain.ProspectSuggestion) []prospectSuggestionResponse {
	out := make([]prospectSuggestionResponse, 0, len(suggestions))
	for _, s := range suggestions {
		out = append(out, prospectSuggestionResponse{
			ProspectID:       s.ProspectID.String(),
			Name:             s.Name,
			Company:          s.Company,
			Email:            s.Email,
			TelegramUsername: s.TelegramUsername,
			SourceName:       s.SourceName,
			Status:           s.Status,
			Confidence:       string(s.Confidence),
		})
	}
	return out
}

// writeSuggestionError maps domain errors to the right HTTP status and a
// fixed public message. Internal error details are NOT echoed to the client
// (which could leak SQL state or schema info); full errors are only surfaced
// via stderr logs — see the standard library default http.Error behavior.
func writeSuggestionError(w http.ResponseWriter, err error, fallbackMsg string) {
	switch {
	case errors.Is(err, domain.ErrLeadNotFound):
		httputil.WriteError(w, http.StatusNotFound, "lead not found")
	case errors.Is(err, domain.ErrProspectNotFound):
		httputil.WriteError(w, http.StatusNotFound, "prospect not found")
	default:
		httputil.WriteError(w, http.StatusInternalServerError, fallbackMsg)
	}
}

func (h *Handler) getProspectSuggestions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		leadID, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		suggestions, err := h.uc.GetProspectSuggestions(r.Context(), userID, leadID)
		if err != nil {
			writeSuggestionError(w, err, "failed to get suggestions")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, suggestionsToResponse(suggestions))
	}
}

func (h *Handler) linkProspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		leadID, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		var body struct {
			ProspectID string `json:"prospect_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		prospectID, err := uuid.Parse(body.ProspectID)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid prospect_id")
			return
		}
		if err := h.uc.LinkProspectToLead(r.Context(), userID, leadID, prospectID); err != nil {
			writeSuggestionError(w, err, "failed to link prospect")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "linked"})
	}
}

func (h *Handler) dismissProspectSuggestion() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		leadID, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		var body struct {
			ProspectID string `json:"prospect_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		prospectID, err := uuid.Parse(body.ProspectID)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid prospect_id")
			return
		}
		if err := h.uc.DismissProspectSuggestion(r.Context(), userID, leadID, prospectID); err != nil {
			writeSuggestionError(w, err, "failed to dismiss suggestion")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "dismissed"})
	}
}

func (h *Handler) suggestionCounts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		counts, err := h.uc.SuggestionCounts(r.Context(), userID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to get suggestion counts")
			return
		}
		// Serialize as {lead_id_string: count} for frontend convenience.
		out := make(map[string]int, len(counts))
		for leadID, n := range counts {
			out[leadID.String()] = n
		}
		httputil.WriteJSON(w, http.StatusOK, out)
	}
}
