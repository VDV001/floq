package leads

import (
	"context"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/daniil/floq/internal/normalize"
	"github.com/google/uuid"
)

// identityResolver is the deterministic IdentityResolver implementation.
// It canonicalizes the raw inputs through internal/normalize, walks the
// fixed priority chain email > phone > tg, and falls back to creating a
// new Identity via domain.NewIdentity (which re-runs the same normalize
// kernel — kept symmetric so the saved value is bit-identical to the
// lookup key).
type identityResolver struct {
	repo domain.IdentityRepository
}

// NewIdentityResolver wires the deterministic resolver. The returned
// value satisfies the domain.IdentityResolver port and is safe for
// concurrent use as long as the underlying repository is.
func NewIdentityResolver(repo domain.IdentityRepository) domain.IdentityResolver {
	return &identityResolver{repo: repo}
}

func (r *identityResolver) Resolve(ctx context.Context, userID uuid.UUID, email, phone, telegramUsername string) (*domain.Identity, error) {
	e := normalize.Email(email)
	p := normalize.Phone(phone)
	tg := normalize.TelegramUsername(telegramUsername)
	if e == "" && p == "" && tg == "" {
		return nil, domain.ErrIdentityNoIdentifiers
	}

	if e != "" {
		existing, err := r.repo.FindByEmail(ctx, userID, e)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}
	if p != "" {
		existing, err := r.repo.FindByPhone(ctx, userID, p)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}
	if tg != "" {
		existing, err := r.repo.FindByTelegramUsername(ctx, userID, tg)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}

	id, err := domain.NewIdentity(userID, e, p, tg)
	if err != nil {
		return nil, err
	}
	if err := r.repo.Save(ctx, id); err != nil {
		return nil, err
	}
	return id, nil
}
