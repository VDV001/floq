package inbox

import (
	"errors"

	"github.com/google/uuid"
)

// BulkDecision enumerates the two terminal decisions the operator can
// apply en-masse to a slice of pending replies. The single-row Approve
// and Reject usecases each map to one of these values; nothing else is
// valid in the bulk API today.
type BulkDecision string

const (
	BulkDecisionApprove BulkDecision = "approve"
	BulkDecisionReject  BulkDecision = "reject"
)

// IsValid reports whether the decision matches a known BulkDecision.
func (d BulkDecision) IsValid() bool {
	switch d {
	case BulkDecisionApprove, BulkDecisionReject:
		return true
	default:
		return false
	}
}

// ErrBulkDecideEmptyIDs rejects bulk calls with an empty id slice —
// the handler answers 400 because no work to do is more likely a UI
// bug than a valid intent. Sentinel so the handler can distinguish
// "bad request shape" from "per-row failure".
var ErrBulkDecideEmptyIDs = errors.New("bulk decide: ids must be non-empty")

// ErrBulkDecideInvalidDecision rejects bulk calls with an unknown
// decision string. Sentinel so the handler can answer 400 with a
// clear message instead of conflating with per-row failures.
var ErrBulkDecideInvalidDecision = errors.New("bulk decide: decision must be 'approve' or 'reject'")

// BulkDecideResult is the per-row outcome of a BulkDecide call. Err
// is nil on success; otherwise it carries the same sentinel the
// single-row usecase returns (ErrPendingReplyNotFound,
// ErrPendingReplyAlreadyDecided, dispatcher failures, …) so the
// handler can map it to a stable wire string without reconstructing
// taxonomy. The slice returned by the usecase preserves input order
// 1-to-1 so callers can correlate by index.
type BulkDecideResult struct {
	ID  uuid.UUID
	Err error
}
