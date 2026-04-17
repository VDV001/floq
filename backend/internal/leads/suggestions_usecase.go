package leads

import (
	"context"
	"fmt"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
)

// GetProspectSuggestions returns candidate cross-channel matches for the lead.
// Returns domain.ErrLeadNotFound if the lead does not belong to the user.
// Returns an empty slice when the feature is disabled (no finder configured).
func (uc *UseCase) GetProspectSuggestions(ctx context.Context, userID, leadID uuid.UUID) ([]domain.ProspectSuggestion, error) {
	if uc.suggestionFinder == nil {
		return []domain.ProspectSuggestion{}, nil
	}
	return uc.suggestionFinder.FindForLead(ctx, userID, leadID)
}

// SuggestionCounts returns the map of lead_id → pending-suggestion count
// for the given user, used by the inbox list to render per-card indicators.
// Returns an empty map if no finder is configured.
func (uc *UseCase) SuggestionCounts(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]int, error) {
	if uc.suggestionFinder == nil {
		return map[uuid.UUID]int{}, nil
	}
	return uc.suggestionFinder.CountsForUser(ctx, userID)
}

// LinkProspectToLead marks the prospect as converted to the lead and — per
// the Lead.InheritsSourceFrom rule — propagates the prospect's source_id onto
// the lead when the lead has none. Atomicity and ownership checks are the
// adapter's responsibility (see cmd/server/adapters.go).
// Returns ErrLeadNotFound / ErrProspectNotFound on ownership mismatch.
func (uc *UseCase) LinkProspectToLead(ctx context.Context, userID, leadID, prospectID uuid.UUID) error {
	if uc.suggestionFinder == nil {
		return fmt.Errorf("suggestion finder not configured")
	}
	return uc.suggestionFinder.LinkProspect(ctx, userID, leadID, prospectID)
}

// DismissProspectSuggestion records a manual rejection so the pair won't
// resurface as a suggestion again.
func (uc *UseCase) DismissProspectSuggestion(ctx context.Context, userID, leadID, prospectID uuid.UUID) error {
	if uc.suggestionFinder == nil {
		return fmt.Errorf("suggestion finder not configured")
	}
	return uc.suggestionFinder.DismissSuggestion(ctx, userID, leadID, prospectID)
}
