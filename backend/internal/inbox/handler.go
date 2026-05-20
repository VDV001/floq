package inbox

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
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
	ListPendingByUser(ctx context.Context, userID uuid.UUID) ([]*PendingReplyWithLead, error)
	Approve(ctx context.Context, userID, id uuid.UUID) error
	Reject(ctx context.Context, userID, id uuid.UUID) error
	UpdateBody(ctx context.Context, userID, id uuid.UUID, body string) (*PendingReply, error)
	BulkDecide(ctx context.Context, userID uuid.UUID, ids []uuid.UUID, decision BulkDecision) ([]BulkDecideResult, error)
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
// decideMW, when non-nil, is applied ONLY to the two mutating routes
// (approve/reject). The GET listing path is intentionally not rate-
// limited: it is idempotent and refreshing it in the UI must not be
// throttled. Passing nil disables the middleware entirely — useful in
// handler unit tests that do not need a Limiter wired up.
//
// A future POST /api/leads/{id}/pending-replies could let operators
// propose a draft manually; for now only the inbox auto-draft path
// (Telegram booking link) feeds the queue.
func RegisterPendingReplyRoutes(r chi.Router, uc PendingReplyUseCaseAPI, leads LeadOwnershipChecker, decideMW func(http.Handler) http.Handler) {
	h := &pendingReplyHandler{uc: uc, leads: leads}
	r.Get("/api/leads/{id}/pending-replies", h.listByLead())
	// Operator queue: all pending rows for the current user across
	// every lead. Idempotent + read-only — not rate-limited.
	r.Get("/api/pending-replies", h.listPendingByUser())
	if decideMW == nil {
		r.Post("/api/pending-replies/{id}/approve", h.approve())
		r.Post("/api/pending-replies/{id}/reject", h.reject())
		r.Patch("/api/pending-replies/{id}", h.updateBody())
		r.Post("/api/pending-replies/bulk", h.bulkDecide())
		return
	}
	r.With(decideMW).Post("/api/pending-replies/{id}/approve", h.approve())
	r.With(decideMW).Post("/api/pending-replies/{id}/reject", h.reject())
	// PATCH is rate-limited too — operators editing in a tight loop should
	// hit the same throttle as a tight approve loop.
	r.With(decideMW).Patch("/api/pending-replies/{id}", h.updateBody())
	// Bulk decide consumes one rate-limit slot per call regardless of
	// how many ids it carries. This is a deliberate power-operator
	// affordance: the alternative — charging N slots per bulk — would
	// neutralise the whole point of the endpoint. If abuse becomes a
	// problem, swap to len(ids) accounting in the middleware key fn.
	r.With(decideMW).Post("/api/pending-replies/bulk", h.bulkDecide())
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

// listPendingByUser answers the operator queue: every status='pending'
// row across every lead the operator owns, joined with the minimum
// lead snippet the frontend needs to render contact + company without
// an N+1 lookup.
//
// The ?status query param is optional. When absent it defaults to
// pending so bare /api/pending-replies works for the queue page. When
// present it MUST be 'pending' — answering 400 to anything else keeps
// the door open for a future widening (e.g. ?status=approved for a
// "recent decisions" tab) without silently filtering today.
func (h *pendingReplyHandler) listPendingByUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		switch r.URL.Query().Get("status") {
		case "", "pending":
			// accepted
		default:
			httputil.WriteError(w, http.StatusBadRequest, "unsupported status filter (only 'pending' is supported)")
			return
		}
		rows, err := h.uc.ListPendingByUser(r.Context(), userID)
		if err != nil {
			slog.ErrorContext(r.Context(), "pending reply list-by-user failed",
				slog.String("user_id", userID.String()),
				slog.Any("err", err))
			httputil.WriteError(w, http.StatusInternalServerError, "failed to list pending replies")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, operatorQueueToResponse(rows))
	}
}

// bulkDecideRequest is the JSON request body. Both fields are
// required; emptiness / unknown decision is detected by the usecase
// and surfaced as a 400, while malformed JSON / non-uuid ids are
// rejected at parse time without reaching the usecase.
type bulkDecideRequest struct {
	IDs      []string `json:"ids"`
	Decision string   `json:"decision"`
}

// bulkDecideResultWire mirrors the per-row outcome on the wire.
// `error` is omitempty so success rows carry only {id, ok:true},
// keeping the response compact for the typical happy-path bulk.
type bulkDecideResultWire struct {
	ID    string `json:"id"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// bulkDecideResponse wraps the per-row results under a "results" key
// so the response shape is forward-compatible with future top-level
// metadata (counts, partial-failure summaries) without breaking
// existing clients.
type bulkDecideResponse struct {
	Results []bulkDecideResultWire `json:"results"`
}

// perRowErrorString maps a per-row usecase error to a stable wire
// string. Unknown errors collapse to a generic "internal error" so
// upstream taxonomy (dispatcher 5xx, db hiccup, …) doesn't leak.
func perRowErrorString(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrPendingReplyNotFound):
		return "not found"
	case errors.Is(err, ErrPendingReplyAlreadyDecided):
		return "already decided"
	case errors.Is(err, ErrPendingReplyDispatcherNotConfigured):
		return "dispatcher not configured"
	default:
		return "internal error"
	}
}

func (h *pendingReplyHandler) bulkDecide() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		var req bulkDecideRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		ids := make([]uuid.UUID, 0, len(req.IDs))
		for _, raw := range req.IDs {
			parsed, err := uuid.Parse(raw)
			if err != nil {
				httputil.WriteError(w, http.StatusBadRequest, "invalid id in ids[]")
				return
			}
			ids = append(ids, parsed)
		}
		results, err := h.uc.BulkDecide(r.Context(), userID, ids, BulkDecision(req.Decision))
		if err != nil {
			switch {
			case errors.Is(err, ErrBulkDecideEmptyIDs):
				httputil.WriteError(w, http.StatusBadRequest, "ids must be non-empty")
			case errors.Is(err, ErrBulkDecideInvalidDecision):
				httputil.WriteError(w, http.StatusBadRequest, "decision must be 'approve' or 'reject'")
			default:
				slog.ErrorContext(r.Context(), "pending reply bulk decide failed",
					slog.String("user_id", userID.String()),
					slog.Int("ids_count", len(ids)),
					slog.Any("err", err))
				httputil.WriteError(w, http.StatusInternalServerError, "failed to process bulk decide")
			}
			return
		}
		out := bulkDecideResponse{Results: make([]bulkDecideResultWire, 0, len(results))}
		for _, res := range results {
			out.Results = append(out.Results, bulkDecideResultWire{
				ID:    res.ID.String(),
				OK:    res.Err == nil,
				Error: perRowErrorString(res.Err),
			})
		}
		httputil.WriteJSON(w, http.StatusOK, out)
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

// updateBodyRequest is the PATCH request body. Body is required;
// presence/non-empty is verified at parse time so the handler can
// answer 400 without a round-trip to the usecase.
type updateBodyRequest struct {
	Body *string `json:"body"`
}

func (h *pendingReplyHandler) updateBody() http.HandlerFunc {
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
		var req updateBodyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if req.Body == nil {
			// Distinguish "missing field" from "empty string" so callers
			// who forgot the field get a clear hint vs. domain-empty.
			httputil.WriteError(w, http.StatusBadRequest, "body field is required")
			return
		}
		updated, err := h.uc.UpdateBody(r.Context(), userID, id, *req.Body)
		if err != nil {
			switch {
			case errors.Is(err, ErrPendingReplyNotFound):
				httputil.WriteError(w, http.StatusNotFound, "pending reply not found")
			case errors.Is(err, ErrPendingReplyAlreadyDecided):
				httputil.WriteError(w, http.StatusConflict, "pending reply already decided")
			case errors.Is(err, ErrPendingReplyEmptyBody):
				httputil.WriteError(w, http.StatusBadRequest, "body is required (non-empty after trim)")
			default:
				slog.ErrorContext(r.Context(), "pending reply update body failed",
					slog.String("pending_reply_id", id.String()),
					slog.String("user_id", userID.String()),
					slog.Any("err", err))
				httputil.WriteError(w, http.StatusInternalServerError, "failed to update pending reply")
			}
			return
		}
		httputil.WriteJSON(w, http.StatusOK, pendingReplyToResponse(updated))
	}
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
				// 500-class errors carry production-actionable
				// detail (dispatcher misconfig, telegram 5xx,
				// db hiccup); log before answering so the
				// operator-visible 500 has a trail to follow.
				slog.ErrorContext(r.Context(), "pending reply decide failed",
					slog.String("pending_reply_id", id.String()),
					slog.String("user_id", userID.String()),
					slog.Any("err", err))
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
//
// DecidedBy is omitempty so rows from before migration 032 (when the
// column did not exist) stay clean on the wire — a missing field
// signals "attribution unknown" rather than synthesising a UUID.
type PendingReplyResponse struct {
	ID        string  `json:"id"`
	LeadID    string  `json:"lead_id"`
	Channel   string  `json:"channel"`
	Kind      string  `json:"kind"`
	Body      string  `json:"body"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
	DecidedAt *string `json:"decided_at,omitempty"`
	DecidedBy *string `json:"decided_by,omitempty"`
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
	if pr.DecidedBy != nil {
		s := pr.DecidedBy.String()
		resp.DecidedBy = &s
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

// LeadSnippetResponse is the wire-shape for the lead context embedded
// in each operator-queue row. Nullable channel-native identifiers are
// omitempty so a telegram lead never carries a JSON null email and
// vice versa — the frontend can branch on field presence.
type LeadSnippetResponse struct {
	ContactName    string  `json:"contact_name"`
	Company        string  `json:"company"`
	Channel        string  `json:"channel"`
	TelegramChatID *int64  `json:"telegram_chat_id,omitempty"`
	EmailAddress   *string `json:"email_address,omitempty"`
}

// OperatorQueueResponse is the wire-shape for a single row in the
// operator queue. Embeds PendingReplyResponse so existing top-level
// fields stay identical to the per-lead endpoint, and adds a nested
// "lead" object so the queue page can render contact + company in one
// pass without N+1.
type OperatorQueueResponse struct {
	PendingReplyResponse
	Lead LeadSnippetResponse `json:"lead"`
}

func leadSnippetToResponse(s LeadSnippet) LeadSnippetResponse {
	return LeadSnippetResponse{
		ContactName:    s.ContactName,
		Company:        s.Company,
		Channel:        string(s.Channel),
		TelegramChatID: s.TelegramChatID,
		EmailAddress:   s.EmailAddress,
	}
}

func operatorQueueToResponse(rows []*PendingReplyWithLead) []OperatorQueueResponse {
	out := make([]OperatorQueueResponse, 0, len(rows))
	for _, row := range rows {
		if row == nil || row.Reply == nil {
			continue
		}
		out = append(out, OperatorQueueResponse{
			PendingReplyResponse: pendingReplyToResponse(row.Reply),
			Lead:                 leadSnippetToResponse(row.Lead),
		})
	}
	return out
}
