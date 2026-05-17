package inbox

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/daniil/floq/internal/httputil"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// PendingReplyUseCaseAPI is the narrow port the HTTP handler needs from
// the usecase. Keeping it as a small interface (instead of taking the
// concrete *PendingReplyUseCase) means the handler test suite can
// stand up without any persistence backend.
type PendingReplyUseCaseAPI interface {
	ListByLead(ctx context.Context, userID, leadID uuid.UUID) ([]*PendingReply, error)
	Approve(ctx context.Context, userID, id uuid.UUID) error
	Reject(ctx context.Context, userID, id uuid.UUID) error
}

// LeadOwnershipChecker is the narrow port the handler needs to gate
// lead-scoped routes. The composition root adapts leads.UseCase.OwnsLead
// to this interface so inbox stays free of leads-package imports on
// the handler edge.
type LeadOwnershipChecker interface {
	OwnsLead(ctx context.Context, userID, leadID uuid.UUID) (bool, error)
}

// pendingReplyHandler binds the usecase and ownership checker to the
// HTTP layer.
type pendingReplyHandler struct {
	uc    PendingReplyUseCaseAPI
	leads LeadOwnershipChecker
}

// RegisterPendingReplyRoutes wires the HITL approval HTTP surface onto
// the given chi router. The set is intentionally narrow:
//   - GET  /api/leads/{id}/pending-replies   — operator queue per lead
//   - POST /api/pending-replies/{id}/approve — approve + dispatch
//   - POST /api/pending-replies/{id}/reject  — terminal reject
//
// A future POST /api/leads/{id}/pending-replies could let operators
// propose a draft manually; for now only the inbox auto-draft path
// (Telegram booking link) feeds the queue.
func RegisterPendingReplyRoutes(r chi.Router, uc PendingReplyUseCaseAPI, leads LeadOwnershipChecker) {
	h := &pendingReplyHandler{uc: uc, leads: leads}
	r.Get("/api/leads/{id}/pending-replies", h.listByLead())
	r.Post("/api/pending-replies/{id}/approve", h.approve())
	r.Post("/api/pending-replies/{id}/reject", h.reject())
}

func (h *pendingReplyHandler) listByLead() http.HandlerFunc {
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
		owned, err := h.leads.OwnsLead(r.Context(), userID, leadID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to authorize lead")
			return
		}
		if !owned {
			httputil.WriteError(w, http.StatusNotFound, "lead not found")
			return
		}
		rows, err := h.uc.ListByLead(r.Context(), userID, leadID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to list pending replies")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, pendingRepliesToResponse(rows))
	}
}

func (h *pendingReplyHandler) approve() http.HandlerFunc {
	return h.decide(func(uc PendingReplyUseCaseAPI, ctx context.Context, userID, id uuid.UUID) error {
		return uc.Approve(ctx, userID, id)
	})
}

func (h *pendingReplyHandler) reject() http.HandlerFunc {
	return h.decide(func(uc PendingReplyUseCaseAPI, ctx context.Context, userID, id uuid.UUID) error {
		return uc.Reject(ctx, userID, id)
	})
}

// decide is the shared shape of Approve and Reject: parse auth, parse
// id, delegate, map sentinel errors to HTTP codes, 204 on success.
func (h *pendingReplyHandler) decide(op func(uc PendingReplyUseCaseAPI, ctx context.Context, userID, id uuid.UUID) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid pending reply id")
			return
		}
		if err := op(h.uc, r.Context(), userID, id); err != nil {
			switch {
			case errors.Is(err, ErrPendingReplyNotFound):
				httputil.WriteError(w, http.StatusNotFound, "pending reply not found")
			case errors.Is(err, ErrPendingReplyAlreadyDecided):
				httputil.WriteError(w, http.StatusConflict, "pending reply already decided")
			default:
				httputil.WriteError(w, http.StatusInternalServerError, "failed to process pending reply")
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- DTO ---

// PendingReplyResponse is the wire-shape returned to the frontend. It
// is intentionally separate from the domain entity so transport-layer
// concerns (JSON tags, timestamp formatting) do not bleed into the
// inbox package surface.
type PendingReplyResponse struct {
	ID        string  `json:"id"`
	LeadID    string  `json:"lead_id"`
	Channel   string  `json:"channel"`
	Kind      string  `json:"kind"`
	Body      string  `json:"body"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
	DecidedAt *string `json:"decided_at,omitempty"`
	SentAt    *string `json:"sent_at,omitempty"`
}

func pendingReplyToResponse(pr *PendingReply) PendingReplyResponse {
	resp := PendingReplyResponse{
		ID:        pr.ID.String(),
		LeadID:    pr.LeadID.String(),
		Channel:   string(pr.Channel),
		Kind:      string(pr.Kind),
		Body:      pr.Body,
		Status:    string(pr.Status),
		CreatedAt: pr.CreatedAt.UTC().Format(time.RFC3339),
	}
	if pr.DecidedAt != nil {
		s := pr.DecidedAt.UTC().Format(time.RFC3339)
		resp.DecidedAt = &s
	}
	if pr.SentAt != nil {
		s := pr.SentAt.UTC().Format(time.RFC3339)
		resp.SentAt = &s
	}
	return resp
}

func pendingRepliesToResponse(rows []*PendingReply) []PendingReplyResponse {
	out := make([]PendingReplyResponse, 0, len(rows))
	for _, pr := range rows {
		out = append(out, pendingReplyToResponse(pr))
	}
	return out
}
