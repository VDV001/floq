package analytics

import (
	"context"
	"errors"

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
