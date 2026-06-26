package prospects

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/prospects/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	uc *UseCase
}

func RegisterRoutes(r chi.Router, uc *UseCase) {
	h := &Handler{uc: uc}
	r.Get("/api/prospects", h.listProspects())
	r.Post("/api/prospects", h.createProspect())
	r.Get("/api/prospects/export", h.exportCSV())
	r.Get("/api/prospects/template", h.templateCSV())
	r.Post("/api/prospects/import", h.importCSV())
	r.Get("/api/prospects/{id}", h.getProspect())
	r.Delete("/api/prospects/{id}", h.deleteProspect())
	r.Post("/api/prospects/{id}/consent", h.setConsent())
}

// setConsent applies an operator's manual consent decision (obtained/withdrawn)
// to a prospect they own.
func (h *Handler) setConsent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid prospect id")
			return
		}
		var body struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		err = h.uc.SetConsent(r.Context(), userID, id, domain.ConsentStatus(body.Status))
		switch {
		case errors.Is(err, ErrProspectNotFound):
			httputil.WriteError(w, http.StatusNotFound, "prospect not found")
		case err != nil:
			// Unsupported status or domain validation — a client-side error.
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			httputil.WriteJSON(w, http.StatusOK, map[string]string{"consent_status": body.Status})
		}
	}
}

func (h *Handler) listProspects() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		prospects, err := h.uc.ListProspects(r.Context(), userID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to list prospects")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, ProspectsToResponse(prospects))
	}
}

func (h *Handler) createProspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var body struct {
			Name             string `json:"name"`
			Company          string `json:"company"`
			Title            string `json:"title"`
			Email            string `json:"email"`
			Phone            string `json:"phone"`
			WhatsApp         string `json:"whatsapp"`
			TelegramUsername string `json:"telegram_username"`
			Industry         string `json:"industry"`
			CompanySize      string `json:"company_size"`
			Context          string     `json:"context"`
			SourceID         *uuid.UUID `json:"source_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		p, err := h.uc.CreateProspect(r.Context(), CreateProspectInput{
			UserID:           userID,
			Name:             body.Name,
			Company:          body.Company,
			Title:            body.Title,
			Email:            body.Email,
			Phone:            body.Phone,
			WhatsApp:         body.WhatsApp,
			TelegramUsername: body.TelegramUsername,
			Industry:         body.Industry,
			CompanySize:      body.CompanySize,
			Context:          body.Context,
			SourceID:         body.SourceID,
		})
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httputil.WriteJSON(w, http.StatusCreated, ProspectToResponse(p))
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
		report, err := h.uc.ImportCSV(r.Context(), userID, data)
		if err != nil {
			msg := err.Error()
			if strings.Contains(msg, "csv header") || strings.Contains(msg, "csv record") {
				httputil.WriteError(w, http.StatusBadRequest, msg)
			} else {
				httputil.WriteError(w, http.StatusBadRequest, "failed to import CSV")
			}
			return
		}
		httputil.WriteJSON(w, http.StatusOK, ImportReportToResponse(report))
	}
}

func (h *Handler) templateCSV() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := h.uc.TemplateCSV()
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="floq-import-template.csv"`)
		w.WriteHeader(http.StatusOK)
		w.Write(data)
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
			httputil.WriteError(w, http.StatusInternalServerError, "failed to export prospects")
			return
		}
		filename := fmt.Sprintf("floq-prospects-%s.csv", time.Now().UTC().Format("2006-01-02"))
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}
}

func (h *Handler) getProspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid prospect id")
			return
		}
		prospect, err := h.uc.GetProspect(r.Context(), id)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to get prospect")
			return
		}
		if prospect == nil {
			httputil.WriteError(w, http.StatusNotFound, "prospect not found")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, ProspectToResponse(prospect))
	}
}

func (h *Handler) deleteProspect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid prospect id")
			return
		}
		if err := h.uc.DeleteProspect(r.Context(), id); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to delete prospect")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
