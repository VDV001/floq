package domain

import (
	"context"

	"github.com/google/uuid"
)

// IdentityRepository persists Identity aggregates and looks them up by
// each canonical identifier. All lookup arguments are expected to be
// pre-normalized (the resolver canonicalizes via internal/normalize
// before calling); SQL implementations should compare byte-exact.
type IdentityRepository interface {
	FindByEmail(ctx context.Context, userID uuid.UUID, email string) (*Identity, error)
	FindByPhone(ctx context.Context, userID uuid.UUID, phone string) (*Identity, error)
	FindByTelegramUsername(ctx context.Context, userID uuid.UUID, tg string) (*Identity, error)
	Save(ctx context.Context, id *Identity) error
}

// IdentityResolver maps a (possibly partial, possibly raw) tuple of
// contact identifiers to a single canonical Identity. When no existing
// identity matches any non-empty input, the resolver constructs and
// persists a fresh one.
//
// Lookup priority is deterministic: email > phone > telegram_username.
// First match wins; subsequent identifiers are not consulted (merge of
// disjoint identities is a Phase-2 concern wired up in PR2).
type IdentityResolver interface {
	Resolve(ctx context.Context, userID uuid.UUID, email, phone, telegramUsername string) (*Identity, error)
}
