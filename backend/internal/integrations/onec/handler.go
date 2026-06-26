package onec

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/go-chi/chi/v5"
)

// webhookSecretHeader carries the per-user secret 1C sends with each call.
const webhookSecretHeader = "X-Onec-Secret"

// Handler exposes the 1C inbound webhook.
type Handler struct {
	uc *UseCase
}

// NewHandler builds a Handler over the use case.
func NewHandler(uc *UseCase) *Handler {
	return &Handler{uc: uc}
}

// RegisterRoutes mounts the public webhook endpoint. It is intentionally
// outside the JWT-protected group: 1C authenticates with its per-user secret
// via WebhookAuth, which resolves the tenant and injects the user id. The
// global JSONBodyCap on the root router already enforces the size limit (413).
func RegisterRoutes(r chi.Router, h *Handler, resolver SecretResolver) {
	r.Group(func(r chi.Router) {
		r.Use(WebhookAuth(resolver))
		r.Post("/api/integrations/onec/webhook", h.Webhook)
	})
}

// webhookRequest is the JSON contract 1C posts. Payload stays raw — its shape
// depends on the 1C configuration and is interpreted by the mapping layer (#107).
type webhookRequest struct {
	ExternalID   string          `json:"external_id"`
	ExternalType string          `json:"external_type"`
	Kind         string          `json:"kind"`
	Payload      json.RawMessage `json:"payload"`
}

// Webhook ingests one 1C event for the authenticated tenant. Replays are
// idempotent and still return 200.
func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req webhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// A body that exceeds the JSONBodyCap is a size violation (413), not
		// malformed JSON (400).
		if httputil.IsBodyTooLarge(err) {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	res, err := h.uc.ProcessInboundEvent(r.Context(), userID, RawInboundEvent{
		ExternalID:   req.ExternalID,
		ExternalType: req.ExternalType,
		Kind:         req.Kind,
		Payload:      req.Payload,
	})
	switch {
	case err == nil:
		// ok
	case errors.Is(err, ErrUnresolvableKind):
		// External type isn't in the user's mapping and no kind was given —
		// Floq can't classify the event.
		http.Error(w, "unmapped event: no kind and no mapping rule", http.StatusUnprocessableEntity)
		return
	case errors.Is(err, domain.ErrInvalidEventKind),
		errors.Is(err, domain.ErrEmptyExternalID),
		errors.Is(err, domain.ErrEmptyExternalType):
		http.Error(w, "invalid event", http.StatusBadRequest)
		return
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "deduped": res.Deduped})
}

// WebhookAuth resolves the tenant from the webhook secret header and injects
// the user id into the request context. An empty or unknown secret → 401; a
// resolver failure → 500.
func WebhookAuth(resolver SecretResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			secret := r.Header.Get(webhookSecretHeader)
			if secret == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			userID, found, err := resolver.UserIDByWebhookSecret(r.Context(), secret)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if !found {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := httputil.WithUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
