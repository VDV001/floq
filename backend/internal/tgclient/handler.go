package tgclient

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/daniil/floq/internal/httputil"
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes adds Telegram personal account endpoints to the router.
// All routes require JWT auth (must be in a protected chi.Group).
func RegisterRoutes(r chi.Router, client *Client, repo *Repository) {
	h := &handler{client: client, repo: repo}

	r.Post("/api/telegram-account/send-code", h.sendCode)
	r.Post("/api/telegram-account/verify", h.verify)
	r.Get("/api/telegram-account/status", h.status)
	r.Delete("/api/telegram-account", h.disconnect)
}

type handler struct {
	client *Client
	repo   *Repository
}

type sendCodeRequest struct {
	Phone string `json:"phone"`
}

type sendCodeResponse struct {
	CodeHash string `json:"code_hash"`
}

type verifyRequest struct {
	Phone    string `json:"phone"`
	Code     string `json:"code"`
	CodeHash string `json:"code_hash"`
}

type statusResponse struct {
	Connected bool   `json:"connected"`
	Phone     string `json:"phone,omitempty"`
}

func (h *handler) sendCode(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req sendCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Phone == "" {
		httputil.WriteError(w, http.StatusBadRequest, "phone is required")
		return
	}

	// Load existing session if any (to reuse connection state).
	_, sessionData, _ := h.repo.GetSession(r.Context(), userID.String())
	if len(sessionData) > 0 {
		h.client.LoadSession(sessionData)
	}

	codeHash, err := h.client.SendCode(r.Context(), req.Phone)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to send code: "+err.Error())
		return
	}

	// Persist intermediate session state so SignIn can resume from same DC.
	if data := h.client.GetSessionData(); len(data) > 0 {
		_ = h.repo.SaveSession(r.Context(), userID.String(), req.Phone, data)
	}

	httputil.WriteJSON(w, http.StatusOK, sendCodeResponse{CodeHash: codeHash})
}

func (h *handler) verify(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Phone == "" || req.Code == "" || req.CodeHash == "" {
		httputil.WriteError(w, http.StatusBadRequest, "phone, code, and code_hash are required")
		return
	}

	// Load session from DB (must have been saved during send-code).
	_, sessionData, _ := h.repo.GetSession(r.Context(), userID.String())
	if len(sessionData) > 0 {
		h.client.LoadSession(sessionData)
	}

	if err := h.client.SignIn(r.Context(), req.Phone, req.Code, req.CodeHash); err != nil {
		log.Printf("[tgclient] sign in error for %s: %v", req.Phone, err)
		httputil.WriteError(w, http.StatusBadRequest, "sign in failed: "+err.Error())
		return
	}

	// Save authenticated session to DB.
	data := h.client.GetSessionData()
	if err := h.repo.SaveSession(r.Context(), userID.String(), req.Phone, data); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to save session")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handler) status(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	phone, sessionData, err := h.repo.GetSession(r.Context(), userID.String())
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to get session")
		return
	}

	connected := len(sessionData) > 0 && phone != ""
	httputil.WriteJSON(w, http.StatusOK, statusResponse{Connected: connected, Phone: phone})
}

func (h *handler) disconnect(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.repo.DeleteSession(r.Context(), userID.String()); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "failed to delete session")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
