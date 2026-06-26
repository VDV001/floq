package domain

import (
	"context"

	"github.com/google/uuid"
)

// IdentityRepository persists Identity aggregates, looks them up by
// each canonical identifier, and maintains the link tables that map
// leads and prospects to the unified identity. All lookup arguments
// are expected to be pre-normalized (the resolver canonicalizes via
// internal/normalize before calling); SQL implementations should
// compare byte-exact.
//
// LinkLead and LinkProspect MUST be idempotent — repeated calls with
// the same arguments produce a single row, so backfill goroutines can
// re-run without duplicating links.
//
// Reader methods (GetByLeadID, LinkedLeadIDs) return (nil, nil) /
// empty slice on missing rows — sentinel errors are reserved for IO
// failures, not for "no such association".
type IdentityRepository interface {
	FindByEmail(ctx context.Context, userID uuid.UUID, email string) (*Identity, error)
	FindByPhone(ctx context.Context, userID uuid.UUID, phone string) (*Identity, error)
	FindByTelegramUsername(ctx context.Context, userID uuid.UUID, tg string) (*Identity, error)
	Save(ctx context.Context, id *Identity) error
	LinkLead(ctx context.Context, leadID, identityID uuid.UUID) error
	LinkProspect(ctx context.Context, prospectID, identityID uuid.UUID) error

	// GetByLeadID returns the Identity linked to the given lead, or
	// (nil, nil) if the lead has no identity attached.
	GetByLeadID(ctx context.Context, leadID uuid.UUID) (*Identity, error)
	// LinkedLeadIDs lists every lead pointing at the identity (including
	// the original triggering lead, if any). Order is unspecified.
	LinkedLeadIDs(ctx context.Context, identityID uuid.UUID) ([]uuid.UUID, error)
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
