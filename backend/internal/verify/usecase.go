package verify

import (
	"context"
	"encoding/json"
	"time"

	"github.com/daniil/floq/internal/prospects/domain"
	"github.com/daniil/floq/internal/proxy"
	"github.com/google/uuid"
)

// ProspectStore is the prospect-repository port used by the verification
// usecase. The composition root binds it to the prospects/Repository.
type ProspectStore interface {
	ListProspects(ctx context.Context, userID uuid.UUID) ([]domain.ProspectWithSource, error)
	GetProspect(ctx context.Context, id uuid.UUID) (*domain.Prospect, error)
	UpdateVerification(ctx context.Context, id uuid.UUID, status domain.VerifyStatus, score int, details string, verifiedAt time.Time) error
}

// TelegramVerifier checks whether a Telegram username corresponds to an
// existing account. The concrete adapter (BotTelegramVerifier in
// telegram.go) wraps the tgbotapi SDK; usecase code depends only on this
// interface to keep the SDK out of the application layer.
type TelegramVerifier interface {
	Verify(username string) TelegramResult
}

// UseCase orchestrates email and Telegram verification across the user's
// prospects. It is the layer that owns the time/iteration/persist policy
// of the verification flow; HTTP handlers delegate to it without making
// any business decisions of their own.
type UseCase struct {
	prospects ProspectStore
	telegram  TelegramVerifier    // optional; nil disables Telegram verification
	dialer    proxy.ContextDialer // optional; nil means direct connections
}

// NewUseCase wires a UseCase against the given prospect store, optional
// Telegram verifier, and optional proxy dialer.
func NewUseCase(prospects ProspectStore, telegram TelegramVerifier, dialer proxy.ContextDialer) *UseCase {
	return &UseCase{prospects: prospects, telegram: telegram, dialer: dialer}
}

// VerifyBatch verifies every not-yet-checked prospect for userID and
// returns the count of prospects whose verification result was persisted.
// Per-prospect failures (JSON marshal, repo update) are logged at the
// caller level and do not abort the batch — the count reflects only the
// successfully persisted entries.
func (uc *UseCase) VerifyBatch(ctx context.Context, userID uuid.UUID) (int, error) {
	list, err := uc.prospects.ListProspects(ctx, userID)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, p := range list {
		if p.VerifyStatus != domain.VerifyStatusNotChecked {
			continue
		}

		emailResult := VerifyEmail(ctx, p.Email, uc.dialer)

		details := map[string]any{
			"email": emailResult,
		}
		if p.TelegramUsername != "" && uc.telegram != nil {
			details["telegram"] = uc.telegram.Verify(p.TelegramUsername)
		}

		detailsJSON, err := json.Marshal(details)
		if err != nil {
			continue
		}

		if err := uc.prospects.UpdateVerification(
			ctx,
			p.ID,
			emailResult.Status,
			emailResult.Score,
			string(detailsJSON),
			time.Now().UTC(),
		); err != nil {
			continue
		}
		count++
	}
	return count, nil
}

// VerifyEmailSingle returns the verification result for one email,
// honoring the usecase's configured proxy dialer.
func (uc *UseCase) VerifyEmailSingle(ctx context.Context, email string) EmailResult {
	return VerifyEmail(ctx, email, uc.dialer)
}

// GetVerifyStatus returns the verification snapshot for a prospect, or
// (nil, nil) if it does not exist.
func (uc *UseCase) GetVerifyStatus(ctx context.Context, id uuid.UUID) (*domain.Prospect, error) {
	return uc.prospects.GetProspect(ctx, id)
}
