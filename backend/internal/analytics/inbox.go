package analytics

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// scoreBuckets defines the qualification-score histogram bands on the
// 0-100 scale (domain ClampScore enforces [0,100]). Lo/Hi are inclusive.
// Declared here, not in the repo, so the SQL FILTER ranges and the DTO
// labels can never drift apart — one source of truth for both.
//
// Note: issue #97 illustrates a 0-10 histogram (0-2 / 3-5 / 6-8 / 9-10),
// but the real qualifications.score column is 0-100. The bands below
// reflect the actual schema; the deviation is intentional.
var scoreBuckets = []struct {
	Lo, Hi int
	Label  string
}{
	{0, 20, "0-20"},
	{21, 40, "21-40"},
	{41, 60, "41-60"},
	{61, 80, "61-80"},
	{81, 100, "81-100"},
}

// ScoreBucketDTO is one bar of the qualification-score histogram. Range
// is the inclusive label ("0-20"); Count is how many leads qualified in
// the period fell in that band.
type ScoreBucketDTO struct {
	Range string
	Count int
}

// LeadsBreakdownDTO is the lead-volume slice of View 3: the period total
// plus per-channel and per-status counts. Maps keep the wire shape close
// to the dashboard ({"telegram": n, ...}) without enumerating every enum
// member as a struct field. Both maps are non-nil (possibly empty) so
// they serialise as {} rather than null.
type LeadsBreakdownDTO struct {
	Total     int
	ByChannel map[string]int
	ByStatus  map[string]int
}

// QualificationDistributionDTO holds the score histogram and the mean
// score over the period. AvgScore is 0 when no leads were qualified.
type QualificationDistributionDTO struct {
	ScoreHistogram []ScoreBucketDTO
	AvgScore       float64
}

// PendingRepliesStatsDTO is the HITL approval slice. Approved counts the
// approved+sent terminal states (both are operator approvals); Rejected
// the rejected state; CurrentlyPending the still-undecided queue. P50/P95
// are the time-to-decide percentiles in whole seconds over decided rows.
//
// The approve-rate is derived at the wire boundary from Approved/Rejected
// so the DTO stays count-pure. There is no edited-then-approved metric:
// pending_replies stores no original body to diff against, so issue #97's
// "edited_then_approved" is dropped in v1 (the issue allows this).
type PendingRepliesStatsDTO struct {
	Approved               int
	Rejected               int
	CurrentlyPending       int
	P50TimeToDecideSeconds int
	P95TimeToDecideSeconds int
}

// InboxFlowDTO is the read model for View 3 — the inbound funnel lens.
// Pure projection over leads + qualifications + pending_replies for the
// [PeriodFrom, PeriodTo) window; no domain entities.
type InboxFlowDTO struct {
	PeriodFrom     time.Time
	PeriodTo       time.Time
	Leads          LeadsBreakdownDTO
	Qualifications QualificationDistributionDTO
	PendingReplies PendingRepliesStatsDTO
}

// InboxFlowReader is the port the usecase depends on for View 3. The pg
// implementation lives in repository.go; tests stub it directly. The
// [from, to) window is resolved at the handler boundary from the period
// query param so this layer stays time-source agnostic (mirrors the
// cost-ratios reader).
type InboxFlowReader interface {
	GetInboxFlow(ctx context.Context, userID uuid.UUID, from, to time.Time) (*InboxFlowDTO, error)
}
