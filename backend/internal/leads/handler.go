package leads

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/leads/domain"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	uc *UseCase
}

func RegisterRoutes(r chi.Router, uc *UseCase) {
	h := &Handler{uc: uc}
	r.Get("/api/leads", h.listLeads())
	r.Get("/api/leads/export", h.exportCSV())
	r.Post("/api/leads/import", h.importCSV())
	r.Get("/api/leads/{id}", h.getLead())
	r.Patch("/api/leads/{id}/status", h.updateStatus())
	r.Get("/api/leads/{id}/messages", h.listMessages())
	r.Post("/api/leads/{id}/send", h.sendMessage())
	r.Get("/api/leads/{id}/qualification", h.getQualification())
	r.Post("/api/leads/{id}/qualify", h.qualifyLead())
	r.Get("/api/leads/{id}/draft", h.getDraft())
	r.Post("/api/leads/{id}/draft/regen", h.regenerateDraft())
}

func (h *Handler) listLeads() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		leads, err := h.uc.ListLeads(r.Context(), userID)
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

func (h *Handler) getLead() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		lead, err := h.uc.GetLead(r.Context(), id)
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

func (h *Handler) updateStatus() http.HandlerFunc {
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

		if err := h.uc.UpdateStatus(r.Context(), id, body.Status); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": body.Status})
	}
}

func (h *Handler) listMessages() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		msgs, err := h.uc.GetMessages(r.Context(), id)
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

func (h *Handler) sendMessage() http.HandlerFunc {
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

		msg, err := h.uc.SendMessage(r.Context(), id, body.Body)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to send message")
			return
		}
		httputil.WriteJSON(w, http.StatusCreated, MessageToResponse(msg))
	}
}

func (h *Handler) getQualification() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		q, err := h.uc.GetQualification(r.Context(), id)
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

func (h *Handler) qualifyLead() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		q, err := h.uc.QualifyLead(r.Context(), id)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to qualify lead")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, QualificationToResponse(q))
	}
}

func (h *Handler) getDraft() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		d, err := h.uc.GetDraft(r.Context(), id)
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

func (h *Handler) regenerateDraft() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		d, err := h.uc.RegenerateDraft(r.Context(), id)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to regenerate draft")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, DraftToResponse(d))
	}
}

func (h *Handler) exportCSV() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		data, err := h.uc.ExportCSV(r.Context(), userID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to export leads")
			return
		}
		filename := fmt.Sprintf("floq-leads-%s.csv", time.Now().UTC().Format("2006-01-02"))
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}
}

func (h *Handler) importCSV() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("file")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "missing file field")
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "failed to read file")
			return
		}

		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		count, err := h.uc.ImportCSV(r.Context(), userID, data)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		httputil.WriteJSON(w, http.StatusOK, map[string]int{"imported": count})
	}
}
