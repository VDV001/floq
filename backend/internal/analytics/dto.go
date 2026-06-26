package analytics

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Period selects the time window for analytics aggregations. Pure value
// type — comparison and serialization come for free. The empty string
// is not a valid period; callers must use ParsePeriod or one of the
// predeclared constants.
type Period string

const (
	PeriodWeek  Period = "week"
	PeriodMonth Period = "month"
	PeriodAll   Period = "all"
)

// ErrInvalidPeriod is returned by ParsePeriod when the caller passes a
// string outside the {week, month, all} set. Sentinel so handlers can
// errors.Is it for the 400-mapping.
var ErrInvalidPeriod = errors.New("invalid analytics period")

// ParsePeriod normalises a query-string value into a typed Period. An
// empty string defaults to PeriodAll so callers don't have to set the
// parameter for the simplest case.
func ParsePeriod(s string) (Period, error) {
	switch s {
	case "":
		return PeriodAll, nil
	case "week":
		return PeriodWeek, nil
	case "month":
		return PeriodMonth, nil
	case "all":
		return PeriodAll, nil
	default:
		return "", ErrInvalidPeriod
	}
}

// SequenceStatsDTO is the read-model row for the sequence-performance
// view. Public fields, no invariants — this is reporting data lifted
// out of outbound_messages + prospects joins, not a domain entity.
type SequenceStatsDTO struct {
	ID        uuid.UUID
	Name      string
	Sent      int64 // outbound rows with status in ('sent', 'bounced') — everything we tried to send
	Delivered int64 // outbound rows with status = 'sent' — passed the bounce check
	Opened    int64 // outbound rows where opened_at IS NOT NULL
	Replied   int64 // outbound rows where replied_at IS NOT NULL
	Converted int64 // DISTINCT prospects with status = 'converted' that received any outbound from this sequence
}

// SequenceStatsReader is the port the usecase depends on. Pg
// implementation lives in repository.go; tests stub it directly.
type SequenceStatsReader interface {
	GetSequenceStats(ctx context.Context, userID uuid.UUID, period Period) ([]SequenceStatsDTO, error)
}

// QualifiedScoreThreshold is the minimum qualifications.score for a
// lead to count as "qualified" in the cost-ratios view. Score range
// is 0-100 (set by the AI qualifier); 80 reflects the threshold the
// product treats as a real sales-ready prospect.
const QualifiedScoreThreshold = 80

// CostRatiosDTO is the read model for the View 2 cost dashboard. All
// money fields are in USD micro-units (integer) at the DTO boundary;
// the wire mapping in the handler converts to float USD with the
// same microToUSD helper the audit package uses, keeping precision
// integer-pure across the aggregation pipeline.
type CostRatiosDTO struct {
	PeriodFrom              time.Time
	PeriodTo                time.Time
	TotalCostUSDMicro       int64
	TotalCalls              int
	LeadsCount              int
	QualifiedLeadsCount     int
	ConvertedCount          int
	DraftsSentCount         int
	CostPerLeadUSDMicro     int64
	CostPerQualifiedUSDMicro int64
	CostPerConvertedUSDMicro int64
	CostPerDraftSentUSDMicro int64
}

// CostRatiosReader is the port the cost-ratios usecase depends on.
// Composes audit_log + leads + qualifications + prospects +
// outbound_messages into a single dashboard row; the pg implementation
// runs the queries in sequence (5 round-trips for v1 — readability
// beats throughput on the operator dashboard).
type CostRatiosReader interface {
	GetCostRatios(ctx context.Context, userID uuid.UUID, from, to time.Time) (*CostRatiosDTO, error)
}
