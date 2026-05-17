// Package domain holds the AuditLogEntry aggregate that captures a
// single AI provider call (Complete or AnalyzeImage). The aggregate is
// constructed by the RecordingProvider decorator after every provider
// round-trip and persisted asynchronously by the audit repository.
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Sentinel errors. NewEntry returns these so callers can route on
// errors.Is — handlers translate them to internal warnings only (we
// never surface audit failures to users, only to metrics).
var (
	ErrInvalidUserID         = errors.New("audit: user_id required")
	ErrInvalidProvider       = errors.New("audit: provider required")
	ErrInvalidModel          = errors.New("audit: model required")
	ErrInvalidRequestType    = errors.New("audit: invalid request_type")
	ErrInvalidStatus         = errors.New("audit: invalid status")
	ErrNegativeTokens        = errors.New("audit: tokens must be non-negative")
	ErrNegativeCost          = errors.New("audit: cost must be non-negative")
	ErrNegativeLatency       = errors.New("audit: latency must be non-negative")
	ErrErrorMessageOnSuccess = errors.New("audit: error_message must be empty when status=success")
	ErrMissingErrorMessage   = errors.New("audit: error_message required when status=error")
)

// RequestType labels the AI use case that issued the underlying provider
// call. Persisted as TEXT in audit_log; the SQL CHECK list in migration
// 028 must stay in sync with this enum.
type RequestType string

const (
	RequestTypeQualification   RequestType = "qualification"
	RequestTypeDraftReply      RequestType = "draft_reply"
	RequestTypeColdMessage     RequestType = "cold_message"
	RequestTypeTelegramMessage RequestType = "telegram_message"
	RequestTypeTelegramReply   RequestType = "telegram_reply"
	RequestTypeCallBrief       RequestType = "call_brief"
	RequestTypeFollowup        RequestType = "followup"
	RequestTypeImageAnalysis   RequestType = "image_analysis"
	RequestTypeStyleCheck      RequestType = "style_check"
	RequestTypeChatAssist      RequestType = "chat_assist"
)

// Status records whether the provider returned a usable response. On
// error we still log latency/model so the spend distribution stays
// honest (failed-but-billed calls do happen with some providers).
type Status string

const (
	StatusSuccess Status = "success"
	StatusError   Status = "error"
)

// Entry is the immutable audit row. Persisted via repository INSERT;
// reads are read-only — no mutator methods because audit history must
// not be amended retroactively (compliance integrity).
type Entry struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	LeadID        *uuid.UUID
	ProspectID    *uuid.UUID
	RequestType   RequestType
	Provider      string
	Model         string
	InputTokens   int
	OutputTokens  int
	TotalTokens   int
	CostUSDMicro  int64
	LatencyMS     int
	Status        Status
	ErrorMessage  string
	CreatedAt     time.Time
}

// EntryParams is the structured input for NewEntry. The factory
// owns ID generation, CreatedAt stamping and total-tokens derivation —
// callers supply only what came back from the provider.
type EntryParams struct {
	UserID       uuid.UUID
	LeadID       *uuid.UUID
	ProspectID   *uuid.UUID
	RequestType  RequestType
	Provider     string
	Model        string
	InputTokens  int
	OutputTokens int
	CostUSDMicro int64
	LatencyMS    int
	Status       Status
	ErrorMessage string
}

// NewEntry validates invariants and assembles an audit row. Returns a
// sentinel error on any violation; on success the caller has a fully
// derived Entry (ID generated, CreatedAt set to UTC now, TotalTokens
// computed) safe to persist.
func NewEntry(p EntryParams) (*Entry, error) {
	if p.UserID == uuid.Nil {
		return nil, ErrInvalidUserID
	}
	if p.Provider == "" {
		return nil, ErrInvalidProvider
	}
	if p.Model == "" {
		return nil, ErrInvalidModel
	}
	if !p.RequestType.valid() {
		return nil, ErrInvalidRequestType
	}
	if !p.Status.valid() {
		return nil, ErrInvalidStatus
	}
	if p.InputTokens < 0 || p.OutputTokens < 0 {
		return nil, ErrNegativeTokens
	}
	if p.CostUSDMicro < 0 {
		return nil, ErrNegativeCost
	}
	if p.LatencyMS < 0 {
		return nil, ErrNegativeLatency
	}
	if p.Status == StatusSuccess && p.ErrorMessage != "" {
		return nil, ErrErrorMessageOnSuccess
	}
	if p.Status == StatusError && p.ErrorMessage == "" {
		return nil, ErrMissingErrorMessage
	}
	return &Entry{
		ID:           uuid.New(),
		UserID:       p.UserID,
		LeadID:       p.LeadID,
		ProspectID:   p.ProspectID,
		RequestType:  p.RequestType,
		Provider:     p.Provider,
		Model:        p.Model,
		InputTokens:  p.InputTokens,
		OutputTokens: p.OutputTokens,
		TotalTokens:  p.InputTokens + p.OutputTokens,
		CostUSDMicro: p.CostUSDMicro,
		LatencyMS:    p.LatencyMS,
		Status:       p.Status,
		ErrorMessage: p.ErrorMessage,
		CreatedAt:    time.Now().UTC(),
	}, nil
}

func (rt RequestType) valid() bool {
	switch rt {
	case RequestTypeQualification, RequestTypeDraftReply, RequestTypeColdMessage,
		RequestTypeTelegramMessage, RequestTypeTelegramReply, RequestTypeCallBrief,
		RequestTypeFollowup, RequestTypeImageAnalysis, RequestTypeStyleCheck,
		RequestTypeChatAssist:
		return true
	}
	return false
}

func (s Status) valid() bool {
	return s == StatusSuccess || s == StatusError
}
