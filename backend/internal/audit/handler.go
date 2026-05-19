package audit

import (
	"errors"
	"net/http"
	"time"

	"github.com/daniil/floq/internal/audit/domain"
	"github.com/daniil/floq/internal/httputil"
	"github.com/go-chi/chi/v5"
)

// Handler exposes audit-aggregation read endpoints. Write path stays
// owned by the async Recorder — handlers never insert into audit_log.
type Handler struct {
	uc *UseCase
}

func NewHandler(uc *UseCase) *Handler {
	return &Handler{uc: uc}
}

// RegisterRoutes wires the audit HTTP surface. Today only the cost-
// summary read endpoint; future endpoints (per-lead breakdown, per-
// prospect drilldown) attach here.
func RegisterRoutes(r chi.Router, h *Handler) {
	r.Get("/api/audit/cost-summary", h.getCostSummary())
}

// getCostSummary handles GET /api/audit/cost-summary?from=YYYY-MM-DD&to=YYYY-MM-DD.
// Both query params are optional; defaults fall back to last 30 days.
func (h *Handler) getCostSummary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		from, ferr := parseDateParam(r, "from")
		if ferr != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid 'from' (want YYYY-MM-DD)")
			return
		}
		to, terr := parseDateParam(r, "to")
		if terr != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid 'to' (want YYYY-MM-DD)")
			return
		}

		summary, err := h.uc.CostSummary(r.Context(), userID, from, to)
		if err != nil {
			if errors.Is(err, ErrInvalidPeriod) {
				httputil.WriteError(w, http.StatusBadRequest, "invalid period: 'to' must be after 'from'")
				return
			}
			httputil.WriteError(w, http.StatusInternalServerError, "failed to aggregate cost summary")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, costSummaryToResponse(summary))
	}
}

// parseDateParam reads a YYYY-MM-DD query parameter. Missing → zero
// time (handler treats as "use default"); malformed → error.
func parseDateParam(r *http.Request, key string) (time.Time, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return time.Time{}, nil
	}
	return time.Parse("2006-01-02", raw)
}

// --- DTO ---

// CostSummaryResponse projects domain.CostSummary onto the wire.
// Micro-USD ints convert to floating-point USD only here; the
// aggregation pipeline keeps integer precision everywhere upstream.
type CostSummaryResponse struct {
	TotalUSD      float64                       `json:"total_usd"`
	TotalCalls    int                           `json:"total_calls"`
	ByRequestType []RequestTypeBreakdownResponse `json:"by_request_type"`
	ByModel       []ModelBreakdownResponse       `json:"by_model"`
	Period        CostSummaryPeriodResponse      `json:"period"`
}

type RequestTypeBreakdownResponse struct {
	RequestType string  `json:"request_type"`
	Calls       int     `json:"calls"`
	USD         float64 `json:"usd"`
	TokensIn    int64   `json:"tokens_in"`
	TokensOut   int64   `json:"tokens_out"`
}

type ModelBreakdownResponse struct {
	Model     string  `json:"model"`
	Calls     int     `json:"calls"`
	USD       float64 `json:"usd"`
	TokensIn  int64   `json:"tokens_in"`
	TokensOut int64   `json:"tokens_out"`
}

type CostSummaryPeriodResponse struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func costSummaryToResponse(s *domain.CostSummary) CostSummaryResponse {
	byType := make([]RequestTypeBreakdownResponse, len(s.ByRequestType))
	for i, b := range s.ByRequestType {
		byType[i] = RequestTypeBreakdownResponse{
			RequestType: b.RequestType,
			Calls:       b.Calls,
			USD:         microToUSD(b.USDMicro),
			TokensIn:    b.InputTokens,
			TokensOut:   b.OutputTokens,
		}
	}
	byModel := make([]ModelBreakdownResponse, len(s.ByModel))
	for i, b := range s.ByModel {
		byModel[i] = ModelBreakdownResponse{
			Model:     b.Model,
			Calls:     b.Calls,
			USD:       microToUSD(b.USDMicro),
			TokensIn:  b.InputTokens,
			TokensOut: b.OutputTokens,
		}
	}
	return CostSummaryResponse{
		TotalUSD:      microToUSD(s.TotalUSDMicro),
		TotalCalls:    s.TotalCalls,
		ByRequestType: byType,
		ByModel:       byModel,
		Period: CostSummaryPeriodResponse{
			From: s.PeriodFrom.UTC().Format("2006-01-02"),
			To:   s.PeriodTo.UTC().Format("2006-01-02"),
		},
	}
}

func microToUSD(m int64) float64 {
	return float64(m) / 1_000_000.0
}
