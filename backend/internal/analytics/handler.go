package analytics

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/daniil/floq/internal/httputil"
	"github.com/go-chi/chi/v5"
)

// Hot-leads limit bounds: default page size and the hard cap that
// prevents an enormous ?limit from sweeping the whole table.
const (
	defaultHotLeadsLimit = 20
	maxHotLeadsLimit     = 100
)

// RegisterRoutes mounts the analytics endpoints onto the chi router.
// All routes require authentication — caller is expected to install
// the auth middleware in the surrounding group.
func RegisterRoutes(r chi.Router, uc *UseCase) {
	h := &handler{uc: uc}
	r.Get("/api/analytics/sequences", h.getSequenceStats)
	r.Get("/api/analytics/cost-ratios", h.getCostRatios)
	r.Get("/api/analytics/hot-leads", h.getHotLeads)
}

type handler struct {
	uc *UseCase
}

// sequenceStatsWire mirrors SequenceStatsDTO onto the JSON wire
// surface. Rates live here (not in the DTO) because they are derived
// presentation values — the DTO stays integer-pure so future callers
// (e.g. cost-attribution joins) can divide on their own units.
type sequenceStatsWire struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Sent           int64   `json:"sent"`
	Delivered      int64   `json:"delivered"`
	Opened         int64   `json:"opened"`
	Replied        int64   `json:"replied"`
	Converted      int64   `json:"converted"`
	OpenRate       float64 `json:"open_rate"`
	ReplyRate      float64 `json:"reply_rate"`
	ConversionRate float64 `json:"conversion_rate"`
}

type sequenceStatsResponse struct {
	Sequences []sequenceStatsWire `json:"sequences"`
	Period    string              `json:"period"`
}

func (h *handler) getSequenceStats(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	period, err := ParsePeriod(r.URL.Query().Get("period"))
	if err != nil {
		if errors.Is(err, ErrInvalidPeriod) {
			httputil.WriteError(w, http.StatusBadRequest, "period must be one of: week, month, all")
			return
		}
		httputil.WriteError(w, http.StatusBadRequest, "invalid period")
		return
	}

	rows, err := h.uc.GetSequenceStats(r.Context(), userID, period)
	if err != nil {
		slog.ErrorContext(r.Context(), "analytics: get sequence stats failed",
			slog.String("user_id", userID.String()),
			slog.String("period", string(period)),
			slog.Any("err", err))
		httputil.WriteError(w, http.StatusInternalServerError, "failed to load analytics")
		return
	}

	wire := make([]sequenceStatsWire, 0, len(rows))
	for _, row := range rows {
		wire = append(wire, sequenceStatsWire{
			ID:             row.ID.String(),
			Name:           row.Name,
			Sent:           row.Sent,
			Delivered:      row.Delivered,
			Opened:         row.Opened,
			Replied:        row.Replied,
			Converted:      row.Converted,
			OpenRate:       safeRatio(row.Opened, row.Delivered),
			ReplyRate:      safeRatio(row.Replied, row.Delivered),
			ConversionRate: safeRatio(row.Converted, row.Delivered),
		})
	}

	httputil.WriteJSON(w, http.StatusOK, sequenceStatsResponse{
		Sequences: wire,
		Period:    string(period),
	})
}

// hotLeadWire mirrors HotLeadDTO onto the JSON surface. Score and
// QualifiedAt are pointers so an unqualified lead serialises them as
// null rather than a misleading zero / zero-time.
type hotLeadWire struct {
	ID             string  `json:"id"`
	ContactName    string  `json:"contact_name"`
	Channel        string  `json:"channel"`
	Status         string  `json:"status"`
	Score          *int    `json:"score"`
	ScoreReason    string  `json:"score_reason"`
	LastActivityAt string  `json:"last_activity_at"`
	QualifiedAt    *string `json:"qualified_at"`
}

type hotLeadsResponse struct {
	Leads         []hotLeadWire `json:"leads"`
	TotalMatching int           `json:"total_matching"`
	LimitApplied  int           `json:"limit_applied"`
}

// parseHotLeadsLimit clamps the ?limit query value into [1, max],
// defaulting on missing/garbage/non-positive input. Clamps rather than
// 400s — a silly limit is a UI nicety, not a client error worth failing.
func parseHotLeadsLimit(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return defaultHotLeadsLimit
	}
	if n > maxHotLeadsLimit {
		return maxHotLeadsLimit
	}
	return n
}

func (h *handler) getHotLeads(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	period, err := ParsePeriod(r.URL.Query().Get("period"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "period must be one of: week, month, all")
		return
	}
	status, err := ParseStatusFilter(r.URL.Query().Get("status"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "status must be one of: any, new, qualified, in_conversation, followup, closed")
		return
	}
	channel, err := ParseChannelFilter(r.URL.Query().Get("channel"))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "channel must be one of: any, telegram, email")
		return
	}
	limit := parseHotLeadsLimit(r.URL.Query().Get("limit"))

	dto, err := h.uc.GetHotLeads(r.Context(), userID, HotLeadsFilter{
		Period:  period,
		Status:  status,
		Channel: channel,
		Limit:   limit,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "analytics: get hot leads failed",
			slog.String("user_id", userID.String()),
			slog.Any("err", err))
		httputil.WriteError(w, http.StatusInternalServerError, "failed to load hot leads")
		return
	}

	wire := make([]hotLeadWire, 0, len(dto.Leads))
	for _, l := range dto.Leads {
		row := hotLeadWire{
			ID:             l.ID.String(),
			ContactName:    l.ContactName,
			Channel:        l.Channel,
			Status:         l.Status,
			Score:          l.Score,
			ScoreReason:    l.ScoreReason,
			LastActivityAt: l.LastActivityAt.UTC().Format(time.RFC3339),
		}
		if l.QualifiedAt != nil {
			qa := l.QualifiedAt.UTC().Format(time.RFC3339)
			row.QualifiedAt = &qa
		}
		wire = append(wire, row)
	}

	httputil.WriteJSON(w, http.StatusOK, hotLeadsResponse{
		Leads:         wire,
		TotalMatching: dto.TotalMatching,
		LimitApplied:  limit,
	})
}

// safeRatio divides numerator by denominator, returning 0 when the
// denominator is non-positive. Used by the rate fields so the JSON
// payload never emits NaN or Inf — those don't round-trip through the
// stdlib encoder.
func safeRatio(num, denom int64) float64 {
	if denom <= 0 {
		return 0
	}
	return float64(num) / float64(denom)
}

// costRatiosPeriodResponse mirrors CostRatiosDTO onto the JSON wire,
// converting USD micro-units to float USD at the boundary.
type costRatiosPeriodResponse struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type costRatiosResponse struct {
	Period                  costRatiosPeriodResponse `json:"period"`
	TotalCostUSD            float64                  `json:"total_cost_usd"`
	TotalCalls              int                      `json:"total_calls"`
	LeadsCount              int                      `json:"leads_count"`
	QualifiedLeadsCount     int                      `json:"qualified_leads_count"`
	ConvertedCount          int                      `json:"converted_count"`
	DraftsSentCount         int                      `json:"drafts_sent_count"`
	CostPerLeadUSD          float64                  `json:"cost_per_lead_usd"`
	CostPerQualifiedLeadUSD float64                  `json:"cost_per_qualified_lead_usd"`
	CostPerConvertedUSD     float64                  `json:"cost_per_converted_usd"`
	CostPerDraftSentUSD     float64                  `json:"cost_per_draft_sent_usd"`
}

// microToUSD mirrors the same conversion the audit package uses so
// the two surfaces report numbers on the same scale.
func microToUSD(m int64) float64 {
	return float64(m) / 1_000_000.0
}

// periodWindow resolves the requested period into a [from, to) pair.
// PeriodAll uses the Unix epoch as the lower bound — a single zero-
// time sentinel would surprise the repo and force every caller to
// branch, so we instead pass an unambiguous "very-old" timestamp.
func periodWindow(period Period, now time.Time) (time.Time, time.Time) {
	to := now
	switch period {
	case PeriodWeek:
		return to.Add(-7 * 24 * time.Hour), to
	case PeriodMonth:
		return to.Add(-30 * 24 * time.Hour), to
	default: // PeriodAll
		return time.Unix(0, 0).UTC(), to
	}
}

func (h *handler) getCostRatios(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	periodParam := r.URL.Query().Get("period")
	if periodParam == "" {
		periodParam = string(PeriodMonth) // default for cost dashboard
	}
	period, err := ParsePeriod(periodParam)
	if err != nil {
		if errors.Is(err, ErrInvalidPeriod) {
			httputil.WriteError(w, http.StatusBadRequest, "period must be one of: week, month, all")
			return
		}
		httputil.WriteError(w, http.StatusBadRequest, "invalid period")
		return
	}

	from, to := periodWindow(period, time.Now().UTC())
	dto, err := h.uc.GetCostRatios(r.Context(), userID, from, to)
	if err != nil {
		slog.ErrorContext(r.Context(), "analytics: get cost ratios failed",
			slog.String("user_id", userID.String()),
			slog.String("period", string(period)),
			slog.Any("err", err))
		httputil.WriteError(w, http.StatusInternalServerError, "failed to load cost analytics")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, costRatiosResponse{
		Period: costRatiosPeriodResponse{
			From: dto.PeriodFrom.UTC().Format(time.RFC3339),
			To:   dto.PeriodTo.UTC().Format(time.RFC3339),
		},
		TotalCostUSD:            microToUSD(dto.TotalCostUSDMicro),
		TotalCalls:              dto.TotalCalls,
		LeadsCount:              dto.LeadsCount,
		QualifiedLeadsCount:     dto.QualifiedLeadsCount,
		ConvertedCount:          dto.ConvertedCount,
		DraftsSentCount:         dto.DraftsSentCount,
		CostPerLeadUSD:          microToUSD(dto.CostPerLeadUSDMicro),
		CostPerQualifiedLeadUSD: microToUSD(dto.CostPerQualifiedUSDMicro),
		CostPerConvertedUSD:     microToUSD(dto.CostPerConvertedUSDMicro),
		CostPerDraftSentUSD:     microToUSD(dto.CostPerDraftSentUSDMicro),
	})
}
