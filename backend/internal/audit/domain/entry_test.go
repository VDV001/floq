package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/daniil/floq/internal/audit/domain"
)

func validParams() domain.EntryParams {
	leadID := uuid.New()
	return domain.EntryParams{
		UserID:       uuid.New(),
		LeadID:       &leadID,
		ProspectID:   nil,
		RequestType:  domain.RequestTypeQualification,
		Provider:     "openai",
		Model:        "gpt-4o-mini",
		InputTokens:  120,
		OutputTokens: 80,
		CostUSDMicro: 18_000, // $0.018
		LatencyMS:    432,
		Status:       domain.StatusSuccess,
		ErrorMessage: "",
	}
}

func TestNewEntry_Success(t *testing.T) {
	t.Parallel()
	p := validParams()
	e, err := domain.NewEntry(p)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if e.ID == uuid.Nil {
		t.Errorf("expected non-zero ID, got %v", e.ID)
	}
	if e.TotalTokens != p.InputTokens+p.OutputTokens {
		t.Errorf("TotalTokens = %d, want %d", e.TotalTokens, p.InputTokens+p.OutputTokens)
	}
	if time.Since(e.CreatedAt) > time.Second {
		t.Errorf("CreatedAt too old: %v", e.CreatedAt)
	}
	if e.Status != domain.StatusSuccess {
		t.Errorf("Status = %q, want %q", e.Status, domain.StatusSuccess)
	}
}

func TestNewEntry_InvariantViolations(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		mutate func(*domain.EntryParams)
		want   error
	}{
		{"missing user_id", func(p *domain.EntryParams) { p.UserID = uuid.Nil }, domain.ErrInvalidUserID},
		{"empty provider", func(p *domain.EntryParams) { p.Provider = "" }, domain.ErrInvalidProvider},
		{"empty model", func(p *domain.EntryParams) { p.Model = "" }, domain.ErrInvalidModel},
		{"unknown request_type", func(p *domain.EntryParams) { p.RequestType = "garbage" }, domain.ErrInvalidRequestType},
		{"unknown status", func(p *domain.EntryParams) { p.Status = "weird" }, domain.ErrInvalidStatus},
		{"negative input tokens", func(p *domain.EntryParams) { p.InputTokens = -1 }, domain.ErrNegativeTokens},
		{"negative output tokens", func(p *domain.EntryParams) { p.OutputTokens = -1 }, domain.ErrNegativeTokens},
		{"negative cost", func(p *domain.EntryParams) { p.CostUSDMicro = -1 }, domain.ErrNegativeCost},
		{"negative latency", func(p *domain.EntryParams) { p.LatencyMS = -1 }, domain.ErrNegativeLatency},
		{
			"error message on success",
			func(p *domain.EntryParams) {
				p.Status = domain.StatusSuccess
				p.ErrorMessage = "should not be here"
			},
			domain.ErrErrorMessageOnSuccess,
		},
		{
			"missing error message on error",
			func(p *domain.EntryParams) {
				p.Status = domain.StatusError
				p.ErrorMessage = ""
			},
			domain.ErrMissingErrorMessage,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := validParams()
			tc.mutate(&p)
			_, err := domain.NewEntry(p)
			if !errors.Is(err, tc.want) {
				t.Fatalf("NewEntry err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNewEntry_AcceptsErrorStatusWithMessage(t *testing.T) {
	t.Parallel()
	p := validParams()
	p.Status = domain.StatusError
	p.ErrorMessage = "rate limit exceeded"
	p.InputTokens = 0
	p.OutputTokens = 0
	p.CostUSDMicro = 0
	e, err := domain.NewEntry(p)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if e.ErrorMessage != "rate limit exceeded" {
		t.Errorf("ErrorMessage = %q, want %q", e.ErrorMessage, "rate limit exceeded")
	}
}

func TestNewEntry_NilLeadAndProspectAllowed(t *testing.T) {
	t.Parallel()
	p := validParams()
	p.LeadID = nil
	p.ProspectID = nil
	_, err := domain.NewEntry(p)
	if err != nil {
		t.Fatalf("nil lead+prospect should be allowed for orphan AI calls, got %v", err)
	}
}

func TestRequestType_AllKnownConstantsValid(t *testing.T) {
	t.Parallel()
	known := []domain.RequestType{
		domain.RequestTypeQualification,
		domain.RequestTypeDraftReply,
		domain.RequestTypeColdMessage,
		domain.RequestTypeTelegramMessage,
		domain.RequestTypeTelegramReply,
		domain.RequestTypeCallBrief,
		domain.RequestTypeFollowup,
		domain.RequestTypeImageAnalysis,
		domain.RequestTypeStyleCheck,
		domain.RequestTypeChatAssist,
	}
	for _, rt := range known {
		t.Run(string(rt), func(t *testing.T) {
			p := validParams()
			p.RequestType = rt
			if _, err := domain.NewEntry(p); err != nil {
				t.Fatalf("known request_type %q rejected: %v", rt, err)
			}
		})
	}
}
