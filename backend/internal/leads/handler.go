package leads

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/leads/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	uc *UseCase
}

// authorizeLead is the shared front-door for every /api/leads/{id}/*
// endpoint: it (a) demands a userID in the request context, (b) parses
// the lead-id path parameter, and (c) gates on UseCase.OwnsLead so a
// foreign tenant gets a uniform 404 — indistinguishable from a
// non-existent leadID, no info leak.
//
// On any failure the helper writes the response and returns ok=false;
// callers must early-return without touching w/r further.
func (h *Handler) authorizeLead(w http.ResponseWriter, r *http.Request) (userID, leadID uuid.UUID, ok bool) {
	userID, present := httputil.UserIDFromContext(r.Context())
	if !present {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return uuid.Nil, uuid.Nil, false
	}
	leadID, err := httputil.ParseIDParam(r, "id")
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
		return uuid.Nil, uuid.Nil, false
	}
	owned, err := h.uc.OwnsLead(r.Context(), userID, leadID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to authorize lead")
		return uuid.Nil, uuid.Nil, false
	}
	if !owned {
		httputil.WriteError(w, http.StatusNotFound, "lead not found")
		return uuid.Nil, uuid.Nil, false
	}
	return userID, leadID, true
}

func RegisterRoutes(r chi.Router, uc *UseCase) {
	h := &Handler{uc: uc}
	r.Get("/api/leads", h.listLeads())
	r.Get("/api/leads/export", h.exportCSV())
	r.Post("/api/leads/import", h.importCSV())
	r.Get("/api/leads/suggestion-counts", h.suggestionCounts())
	r.Get("/api/leads/{id}", h.getLead())
	r.Patch("/api/leads/{id}/status", h.updateStatus())
	r.Post("/api/leads/{id}/archive", h.archiveLead())
	r.Post("/api/leads/{id}/unarchive", h.unarchiveLead())
	r.Get("/api/leads/{id}/messages", h.listMessages())
	r.Post("/api/leads/{id}/send", h.sendMessage())
	r.Get("/api/leads/{id}/qualification", h.getQualification())
	r.Post("/api/leads/{id}/qualify", h.qualifyLead())
	r.Get("/api/leads/{id}/draft", h.getDraft())
	r.Post("/api/leads/{id}/draft/regen", h.regenerateDraft())
	r.Get("/api/leads/{id}/prospect-suggestions", h.getProspectSuggestions())
	r.Post("/api/leads/{id}/link-prospect", h.linkProspect())
	r.Post("/api/leads/{id}/dismiss-prospect-suggestion", h.dismissProspectSuggestion())
}

func (h *Handler) listLeads() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		leads, err := h.uc.ListLeadsWithPendingCounts(r.Context(), userID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to list leads")
			return
		}
		if leads == nil {
			leads = []LeadWithPendingCount{}
		}
		httputil.WriteJSON(w, http.StatusOK, LeadsWithPendingCountToResponse(leads))
	}
}

func (h *Handler) getLead() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid lead id")
			return
		}
		view, err := h.uc.GetLeadView(r.Context(), userID, id)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to get lead")
			return
		}
		if view == nil {
			httputil.WriteError(w, http.StatusNotFound, "lead not found")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, LeadViewToResponse(view))
	}
}

func (h *Handler) updateStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, id, ok := h.authorizeLead(w, r)
		if !ok {
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

// archiveLead hides the lead from feeds/analytics. Ownership is gated by the
// shared authorizeLead front-door (foreign/missing → 404). A double-archive
// surfaces as 409 via domain.ErrAlreadyArchived; archive is orthogonal to
// status, so the lead's pipeline state is untouched.
func (h *Handler) archiveLead() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, id, ok := h.authorizeLead(w, r)
		if !ok {
			return
		}
		if err := h.uc.ArchiveLead(r.Context(), id); err != nil {
			if errors.Is(err, domain.ErrAlreadyArchived) {
				httputil.WriteError(w, http.StatusConflict, "lead already archived")
				return
			}
			httputil.WriteError(w, http.StatusInternalServerError, "failed to archive lead")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "archived"})
	}
}

// unarchiveLead restores an archived lead. A not-archived lead surfaces as 409
// via domain.ErrNotArchived.
func (h *Handler) unarchiveLead() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, id, ok := h.authorizeLead(w, r)
		if !ok {
			return
		}
		if err := h.uc.UnarchiveLead(r.Context(), id); err != nil {
			if errors.Is(err, domain.ErrNotArchived) {
				httputil.WriteError(w, http.StatusConflict, "lead is not archived")
				return
			}
			httputil.WriteError(w, http.StatusInternalServerError, "failed to unarchive lead")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "active"})
	}
}

func (h *Handler) listMessages() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, id, ok := h.authorizeLead(w, r)
		if !ok {
			return
		}

		// ?aggregated=true switches to the identity-merged timeline,
		// where messages from every lead sharing this lead's Identity
		// surface in chronological order. The frontend pins the param
		// to the user_settings.aggregated_inbox_view preference. The
		// aggregated path also re-filters linked leads through the
		// same userID so a future operator-driven cross-tenant merge
		// (Phase 3) cannot expose foreign messages.
		aggregated := r.URL.Query().Get("aggregated") == "true"

		var msgs []domain.Message
		var err error
		if aggregated {
			msgs, err = h.uc.GetAggregatedMessages(r.Context(), userID, id)
		} else {
			msgs, err = h.uc.GetMessages(r.Context(), id)
		}
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
		_, id, ok := h.authorizeLead(w, r)
		if !ok {
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
		_, id, ok := h.authorizeLead(w, r)
		if !ok {
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
		_, id, ok := h.authorizeLead(w, r)
		if !ok {
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
		_, id, ok := h.authorizeLead(w, r)
		if !ok {
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
		_, id, ok := h.authorizeLead(w, r)
		if !ok {
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
			if httputil.IsBodyTooLarge(err) {
				httputil.WriteError(w, http.StatusRequestEntityTooLarge, "uploaded file too large")
				return
			}
			httputil.WriteError(w, http.StatusBadRequest, "missing file field")
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			// Belt-and-suspenders: an oversized body almost always trips the cap
			// earlier in FormFile/ParseMultipartForm, but keep the 413 mapping
			// here too so a streamed read can never surface as a generic 400.
			if httputil.IsBodyTooLarge(err) {
				httputil.WriteError(w, http.StatusRequestEntityTooLarge, "uploaded file too large")
				return
			}
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
