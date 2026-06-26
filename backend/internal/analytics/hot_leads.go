package analytics

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Sentinel errors for the hot-leads query-param validators, so the
// handler can errors.Is them for the 400 mapping (mirrors
// ErrInvalidPeriod).
var (
	ErrInvalidStatusFilter  = errors.New("invalid lead status filter")
	ErrInvalidChannelFilter = errors.New("invalid lead channel filter")
)

// FilterAny is the sentinel meaning "no constraint on this dimension".
const FilterAny = "any"

// ParseStatusFilter normalises the ?status query value. Empty or "any"
// means no status constraint; otherwise the value must be a member of
// the lead_status enum (migration 002). Returns ErrInvalidStatusFilter
// for anything else.
func ParseStatusFilter(s string) (string, error) {
	switch s {
	case "", FilterAny:
		return FilterAny, nil
	case "new", "qualified", "in_conversation", "followup", "closed":
		return s, nil
	default:
		return "", ErrInvalidStatusFilter
	}
}

// ParseChannelFilter normalises the ?channel query value against the
// lead_channel enum. Empty or "any" means no channel constraint.
func ParseChannelFilter(s string) (string, error) {
	switch s {
	case "", FilterAny:
		return FilterAny, nil
	case "telegram", "email":
		return s, nil
	default:
		return "", ErrInvalidChannelFilter
	}
}

// HotLeadsFilter is the validated query input for the hot-leads view.
// Status/Channel are either FilterAny or a valid enum member; Limit is
// already clamped to a sane range by the handler.
type HotLeadsFilter struct {
	Period  Period
	Status  string
	Channel string
	Limit   int
}

// HotLeadDTO is one row of the hot-leads read model. Public fields, no
// invariants — a projection over leads LEFT JOIN qualifications, not a
// domain entity. Score/ScoreReason/QualifiedAt are pointers because a
// lead may not have been qualified yet (the LEFT JOIN yields NULLs).
type HotLeadDTO struct {
	ID             uuid.UUID
	ContactName    string
	Channel        string
	Status         string
	Score          *int
	ScoreReason    string
	LastActivityAt time.Time
	QualifiedAt    *time.Time
}

// HotLeadsDTO wraps the page of rows with the total number of matching
// leads (before LIMIT) and the limit actually applied, so the UI can
// show "showing 20 of 45".
type HotLeadsDTO struct {
	Leads         []HotLeadDTO
	TotalMatching int
	LimitApplied  int
}

// HotLeadsReader is the port the usecase depends on for View 4. The pg
// implementation lives in repository.go; tests stub it directly.
type HotLeadsReader interface {
	GetHotLeads(ctx context.Context, userID uuid.UUID, filter HotLeadsFilter) (*HotLeadsDTO, error)
}
