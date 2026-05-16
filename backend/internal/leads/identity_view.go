package leads

import (
	"context"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
)

// IdentityReader is the narrow port the lead-view use case needs from
// the identity machinery. Kept separate from domain.IdentityRepository
// so the use case does not depend on Save/Find/Link mutations it never
// invokes (ISP).
type IdentityReader interface {
	GetByLeadID(ctx context.Context, leadID uuid.UUID) (*domain.Identity, error)
	LinkedLeadIDs(ctx context.Context, identityID uuid.UUID) ([]uuid.UUID, error)
}

// LeadView is the read-projection returned by GetLeadView: the lead
// itself plus the optional unified-identity context the detail page
// uses to render the IdentityBadge + cross-channel toggles.
//
// Identity is nil when the lead has no linked Identity yet (legacy
// rows that haven't been backfilled, or single-channel leads where
// the user hasn't merged anything). LinkedLeadIDs always includes the
// triggering lead when Identity is non-nil — clients can dedupe.
type LeadView struct {
	Lead          *domain.Lead
	Identity      *domain.Identity
	LinkedLeadIDs []uuid.UUID
}

// WithIdentityReader wires the optional identity reader. Omit (or
// pass nil) to keep GetLeadView in legacy mode: it returns the lead
// with Identity=nil unconditionally.
func WithIdentityReader(r IdentityReader) Option {
	return func(uc *UseCase) { uc.identityReader = r }
}

// GetLeadView returns the lead plus its identity context for the
// detail page. When the identity reader is wired but fails (DB blip,
// timeout), the view degrades to lead-only and the error is logged
// — the detail page must never 500 just because the identity-side
// is unhealthy.
//
// Returns (nil, nil) when the lead does not exist — handler maps to
// 404 to stay symmetric with GetLead.
func (uc *UseCase) GetLeadView(ctx context.Context, leadID uuid.UUID) (*LeadView, error) {
	lead, err := uc.repo.GetLead(ctx, leadID)
	if err != nil {
		return nil, err
	}
	if lead == nil {
		return nil, nil
	}

	view := &LeadView{Lead: lead}
	if uc.identityReader == nil {
		return view, nil
	}

	identity, err := uc.identityReader.GetByLeadID(ctx, leadID)
	if err != nil {
		uc.logger.WarnContext(ctx, "leads: identity fetch failed, falling back to lead-only view",
			"lead", leadID, "err", err)
		return view, nil
	}
	if identity == nil {
		return view, nil
	}
	view.Identity = identity

	linked, err := uc.identityReader.LinkedLeadIDs(ctx, identity.ID)
	if err != nil {
		uc.logger.WarnContext(ctx, "leads: linked lead enumeration failed",
			"lead", leadID, "identity", identity.ID, "err", err)
		// Preserve the identity we already have; just expose no siblings.
		return view, nil
	}
	view.LinkedLeadIDs = linked
	return view, nil
}

