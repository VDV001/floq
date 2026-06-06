package prospects

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/daniil/floq/internal/prospects/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// unsubscribeStore is the narrow persistence port the unsubscribe flow needs.
// The concrete *Repository satisfies it; kept separate from the broad
// domain.Repository so this public, unauthenticated path depends on the minimum.
type unsubscribeStore interface {
	GetProspect(ctx context.Context, id uuid.UUID) (*domain.Prospect, error)
	AddSuppression(ctx context.Context, s *domain.Suppression) error
	UpdateConsent(ctx context.Context, prospectID uuid.UUID, c domain.Consent) error
}

// UnsubscribeService verifies a signed unsubscribe token and honors it:
// suppress the prospect's email and record the consent withdrawal. It is the
// write side of the unsubscribe link carried by every outbound email.
type UnsubscribeService struct {
	store  unsubscribeStore
	secret string
}

// NewUnsubscribeService builds the service. secret signs and verifies tokens —
// the same value the outbound sender uses to mint the links.
func NewUnsubscribeService(store unsubscribeStore, secret string) *UnsubscribeService {
	return &UnsubscribeService{store: store, secret: secret}
}

// Unsubscribe verifies token and, for the prospect it authorizes, suppresses
// the email address and records a withdrawn consent. It is idempotent (repeated
// requests re-apply the same terminal state) and privacy-preserving: a token
// for a prospect that no longer exists is a silent no-op, never an existence
// oracle. Returns domain.ErrInvalidUnsubscribeToken for a bad token.
func (s *UnsubscribeService) Unsubscribe(ctx context.Context, token string) error {
	prospectID, err := domain.ParseUnsubscribeToken(token, s.secret)
	if err != nil {
		return err
	}
	p, err := s.store.GetProspect(ctx, prospectID)
	if err != nil {
		return err
	}
	if p == nil {
		// Token valid but the prospect is gone — nothing to contact. Stay silent
		// so the endpoint can't confirm whether a prospect ID exists.
		return nil
	}

	// Suppress the email first (the hard, address-level block) so even a stale
	// duplicate prospect record with the same email is covered, then record the
	// consent withdrawal on this prospect for its compliance state and the UI.
	if p.Email != "" {
		sup, err := domain.NewSuppression(p.UserID, domain.SuppressionChannelEmail, p.Email, "unsubscribe")
		if err != nil {
			return err
		}
		if err := s.store.AddSuppression(ctx, sup); err != nil {
			return err
		}
	}
	if err := p.WithdrawConsent("unsubscribe", time.Now().UTC()); err != nil {
		return err
	}
	return s.store.UpdateConsent(ctx, p.ID, p.Consent)
}

// HandleUnsubscribe serves the public unsubscribe endpoint for both a user's
// link click (GET) and a mail client's RFC 8058 one-click POST.
func (s *UnsubscribeService) HandleUnsubscribe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")
		err := s.Unsubscribe(r.Context(), token)
		switch {
		case errors.Is(err, domain.ErrInvalidUnsubscribeToken):
			writeUnsubscribePage(w, http.StatusBadRequest, "Ссылка недействительна или устарела.")
		case err != nil:
			log.Printf("[unsubscribe] failed: %v", err)
			writeUnsubscribePage(w, http.StatusInternalServerError, "Не удалось обработать запрос. Попробуйте позже.")
		default:
			writeUnsubscribePage(w, http.StatusOK, "Вы отписаны. Больше писем не придёт.")
		}
	}
}

// RegisterUnsubscribeRoutes mounts the public unsubscribe endpoint. GET handles
// the human link click; POST handles the RFC 8058 List-Unsubscribe-Post
// one-click sent by mail clients. No authentication — the signed token is the
// authorization.
func RegisterUnsubscribeRoutes(r chi.Router, s *UnsubscribeService) {
	r.Get("/unsubscribe/{token}", s.HandleUnsubscribe())
	r.Post("/unsubscribe/{token}", s.HandleUnsubscribe())
}

// writeUnsubscribePage renders a minimal self-contained HTML confirmation.
func writeUnsubscribePage(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`<!doctype html><html lang="ru"><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width,initial-scale=1">` +
		`<title>Отписка</title></head><body style="font-family:system-ui,sans-serif;max-width:32rem;margin:4rem auto;padding:0 1rem;text-align:center">` +
		`<p style="font-size:1.1rem">` + message + `</p></body></html>`))
}
