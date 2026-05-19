package domain

import "time"

// CostSummary is the per-user aggregated spend ledger over a closed
// time range [PeriodFrom, PeriodTo). Returned by AuditRepository.
// CostSummary and surfaced to operators via GET /api/audit/cost-summary.
//
// USD values stay in micro-USD (USD * 1_000_000) all the way through
// the stack and convert only at the JSON edge — integer arithmetic
// avoids accumulated float error on sums of thousands of rows.
type CostSummary struct {
	TotalUSDMicro int64
	TotalCalls    int
	ByRequestType []RequestTypeBreakdown
	ByModel       []ModelBreakdown
	PeriodFrom    time.Time
	PeriodTo      time.Time
}

// RequestTypeBreakdown rolls audit_log rows by request_type. Ordered
// by total spend desc at the SQL layer so the operator dashboard
// shows the most expensive surface first.
type RequestTypeBreakdown struct {
	RequestType  string
	Calls        int
	USDMicro     int64
	InputTokens  int64
	OutputTokens int64
}

// ModelBreakdown rolls audit_log rows by model name, same ordering
// rationale as RequestTypeBreakdown.
type ModelBreakdown struct {
	Model        string
	Calls        int
	USDMicro     int64
	InputTokens  int64
	OutputTokens int64
}
