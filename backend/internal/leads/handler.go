package leads

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func getUserID(r *http.Request) uuid.UUID {
	uid, _ := r.Context().Value("user_id").(uuid.UUID)
	return uid
}

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

func listLeads(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		leads, err := uc.ListLeads(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list leads")
			return
		}
		if leads == nil {
			leads = []Lead{}
		}
		writeJSON(w, http.StatusOK, leads)
	}
}

func getLead(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		lead, err := uc.GetLead(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get lead")
			return
		}
		if lead == nil {
			writeError(w, http.StatusNotFound, "lead not found")
			return
		}
		writeJSON(w, http.StatusOK, lead)
	}
}

func updateStatus(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid lead id")
			return
		}

		var body struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Status == "" {
			writeError(w, http.StatusBadRequest, "status is required")
			return
		}

		if err := uc.UpdateStatus(r.Context(), id, body.Status); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update status")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": body.Status})
	}
}

func listMessages(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		msgs, err := uc.GetMessages(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list messages")
			return
		}
		if msgs == nil {
			msgs = []Message{}
		}
		writeJSON(w, http.StatusOK, msgs)
	}
}

func sendMessage(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid lead id")
			return
		}

		var body struct {
			Body string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Body == "" {
			writeError(w, http.StatusBadRequest, "body is required")
			return
		}

		msg, err := uc.SendMessage(r.Context(), id, body.Body)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to send message")
			return
		}
		writeJSON(w, http.StatusCreated, msg)
	}
}

func getQualification(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		q, err := uc.GetQualification(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get qualification")
			return
		}
		if q == nil {
			writeError(w, http.StatusNotFound, "qualification not found")
			return
		}
		writeJSON(w, http.StatusOK, q)
	}
}

func qualifyLead(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		q, err := uc.QualifyLead(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to qualify lead")
			return
		}
		writeJSON(w, http.StatusOK, q)
	}
}

func getDraft(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		d, err := uc.GetDraft(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get draft")
			return
		}
		if d == nil {
			writeError(w, http.StatusNotFound, "draft not found")
			return
		}
		writeJSON(w, http.StatusOK, d)
	}
}

func regenerateDraft(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		d, err := uc.RegenerateDraft(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to regenerate draft")
			return
		}
		writeJSON(w, http.StatusOK, d)
	}
}
