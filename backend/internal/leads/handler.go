package leads

import (
	"encoding/json"
	"net/http"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/leads/domain"
	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router, uc *UseCase) {
	r.Get("/api/leads", listLeads(uc))
	r.Get("/api/leads/{id}", getLead(uc))
	r.Patch("/api/leads/{id}/status", updateStatus(uc))
	r.Get("/api/leads/{id}/messages", listMessages(uc))
	r.Post("/api/leads/{id}/send", sendMessage(uc))
	r.Get("/api/leads/{id}/qualification", getQualification(uc))
	r.Post("/api/leads/{id}/qualify", qualifyLead(uc))
	r.Get("/api/leads/{id}/draft", getDraft(uc))
	r.Post("/api/leads/{id}/draft/regen", regenerateDraft(uc))
}

func listLeads(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		leads, err := uc.ListLeads(r.Context(), userID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to list leads")
			return
		}
		if leads == nil {
			leads = []domain.Lead{}
		}
		httputil.WriteJSON(w, http.StatusOK, LeadsToResponse(leads))
	}
}

func getLead(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		lead, err := uc.GetLead(r.Context(), id)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to get lead")
			return
		}
		if lead == nil {
			httputil.WriteError(w, http.StatusNotFound, "lead not found")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, LeadToResponse(lead))
	}
}

func updateStatus(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
			return
		}

		var body struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Status == "" {
			httputil.WriteError(w, http.StatusBadRequest, "status is required")
			return
		}

		if err := uc.UpdateStatus(r.Context(), id, body.Status); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to update status")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": body.Status})
	}
}

func listMessages(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		msgs, err := uc.GetMessages(r.Context(), id)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to list messages")
			return
		}
		if msgs == nil {
			msgs = []domain.Message{}
		}
		httputil.WriteJSON(w, http.StatusOK, MessagesToResponse(msgs))
	}
}

func sendMessage(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
			return
		}

		var body struct {
			Body string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Body == "" {
			httputil.WriteError(w, http.StatusBadRequest, "body is required")
			return
		}

		msg, err := uc.SendMessage(r.Context(), id, body.Body)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to send message")
			return
		}
		httputil.WriteJSON(w, http.StatusCreated, MessageToResponse(msg))
	}
}

func getQualification(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		q, err := uc.GetQualification(r.Context(), id)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to get qualification")
			return
		}
		if q == nil {
			httputil.WriteError(w, http.StatusNotFound, "qualification not found")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, QualificationToResponse(q))
	}
}

func qualifyLead(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		q, err := uc.QualifyLead(r.Context(), id)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to qualify lead")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, QualificationToResponse(q))
	}
}

func getDraft(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		d, err := uc.GetDraft(r.Context(), id)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to get draft")
			return
		}
		if d == nil {
			httputil.WriteError(w, http.StatusNotFound, "draft not found")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, DraftToResponse(d))
	}
}

func regenerateDraft(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		d, err := uc.RegenerateDraft(r.Context(), id)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to regenerate draft")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, DraftToResponse(d))
	}
}
