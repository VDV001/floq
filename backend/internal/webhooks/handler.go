package webhooks

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/webhooks/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler serves the webhook subscription management API. Every route is
// tenant-scoped: a caller can only see and mutate their own endpoints.
type Handler struct {
	uc *UseCase
}

// RegisterRoutes mounts the webhook management endpoints under /api/webhooks.
func RegisterRoutes(r chi.Router, uc *UseCase) {
	h := &Handler{uc: uc}
	r.Post("/api/webhooks", h.create())
	r.Get("/api/webhooks", h.list())
	r.Get("/api/webhooks/event-types", h.eventTypes())
	r.Delete("/api/webhooks/{id}", h.delete())
	r.Post("/api/webhooks/{id}/test", h.test())
}

// eventTypes advertises the events a subscription may listen for, so the UI can
// build its picker from the authoritative domain registry rather than a
// hardcoded copy.
func (h *Handler) eventTypes() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		known := domain.KnownEventTypes()
		out := make([]string, len(known))
		for i, et := range known {
			out[i] = string(et)
		}
		httputil.WriteJSON(w, http.StatusOK, out)
	}
}

// createRequest is the create-endpoint payload. The secret is supplied by the
// caller (their receiver must know it to verify signatures) and is never echoed
// back.
type createRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
	Secret string   `json:"secret"`
}

// endpointResponse is the safe projection of an endpoint: it deliberately omits
// the signing secret so the API never leaks it after creation.
type endpointResponse struct {
	ID     uuid.UUID `json:"id"`
	URL    string    `json:"url"`
	Events []string  `json:"events"`
	Active bool      `json:"active"`
}

func toResponse(e *domain.WebhookEndpoint) endpointResponse {
	events := make([]string, len(e.Events))
	for i, et := range e.Events {
		events[i] = string(et)
	}
	return endpointResponse{ID: e.ID, URL: e.URL.String(), Events: events, Active: e.Active}
}

func (h *Handler) create() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		var body createRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		ep, err := h.uc.CreateEndpoint(r.Context(), userID, body.URL, body.Events, body.Secret)
		if err != nil {
			httputil.WriteError(w, mapCreateError(err), createErrorMessage(err))
			return
		}
		httputil.WriteJSON(w, http.StatusCreated, toResponse(ep))
	}
}

func (h *Handler) list() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		endpoints, err := h.uc.ListEndpoints(r.Context(), userID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to list webhooks")
			return
		}
		out := make([]endpointResponse, 0, len(endpoints))
		for _, e := range endpoints {
			out = append(out, toResponse(e))
		}
		httputil.WriteJSON(w, http.StatusOK, out)
	}
}

func (h *Handler) delete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, id, ok := h.authAndID(w, r)
		if !ok {
			return
		}
		if err := h.uc.DeleteEndpoint(r.Context(), userID, id); err != nil {
			h.writeMutationError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) test() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, id, ok := h.authAndID(w, r)
		if !ok {
			return
		}
		if err := h.uc.TestEndpoint(r.Context(), userID, id); err != nil {
			h.writeMutationError(w, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

// authAndID extracts the authenticated user and the {id} path param, writing the
// appropriate error and returning ok=false if either is missing/invalid.
func (h *Handler) authAndID(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		// An unparseable ID can't name a real endpoint; 404 (anti-enumeration).
		httputil.WriteError(w, http.StatusNotFound, "webhook not found")
		return uuid.Nil, uuid.Nil, false
	}
	return userID, id, true
}

// writeMutationError maps usecase errors for delete/test: not-found (incl.
// not-owned) → 404, everything else → 500.
func (h *Handler) writeMutationError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrEndpointNotFound) {
		httputil.WriteError(w, http.StatusNotFound, "webhook not found")
		return
	}
	httputil.WriteError(w, http.StatusInternalServerError, "request failed")
}

// mapCreateError maps domain validation errors to 400 and anything else to 500.
func mapCreateError(err error) int {
	switch {
	case errors.Is(err, domain.ErrInvalidWebhookURL),
		errors.Is(err, domain.ErrUnknownEventType),
		errors.Is(err, domain.ErrNoEvents),
		errors.Is(err, domain.ErrWeakSecret),
		errors.Is(err, domain.ErrEmptyOwner):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func createErrorMessage(err error) string {
	if mapCreateError(err) == http.StatusBadRequest {
		return err.Error()
	}
	return "failed to create webhook"
}
