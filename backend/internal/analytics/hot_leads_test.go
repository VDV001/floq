package analytics_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/daniil/floq/internal/analytics"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubHotLeadsReader records the filter it was called with and returns
// canned rows — keeps the handler test DB-free.
type stubHotLeadsReader struct {
	dto       *analytics.HotLeadsDTO
	err       error
	gotUserID uuid.UUID
	gotFilter analytics.HotLeadsFilter
	calls     int
}

func (s *stubHotLeadsReader) GetHotLeads(_ context.Context, userID uuid.UUID, f analytics.HotLeadsFilter) (*analytics.HotLeadsDTO, error) {
	s.calls++
	s.gotUserID = userID
	s.gotFilter = f
	if s.err != nil {
		return nil, s.err
	}
	return s.dto, nil
}

func hotLeadsRouter(reader analytics.HotLeadsReader) chi.Router {
	r := chi.NewRouter()
	analytics.RegisterRoutes(r, analytics.NewUseCase(nil, nil, analytics.WithHotLeadsReader(reader)))
	return r
}

func TestHandler_GetHotLeads_ReturnsRowsAndPassesFilter(t *testing.T) {
	userID := uuid.New()
	leadID := uuid.New()
	score := 87
	qAt := time.Date(2026, 5, 19, 14, 23, 0, 0, time.UTC)
	reader := &stubHotLeadsReader{dto: &analytics.HotLeadsDTO{
		Leads: []analytics.HotLeadDTO{{
			ID: leadID, ContactName: "Acme Corp", Channel: "telegram",
			Status: "qualified", Score: &score, ScoreReason: "strong fit",
			LastActivityAt: qAt, QualifiedAt: &qAt,
		}},
		TotalMatching: 45,
		LimitApplied:  20,
	}}

	w := httptest.NewRecorder()
	req := newAuthedRequest(t, "/api/analytics/hot-leads?status=qualified&channel=telegram&period=month&limit=20", userID)
	hotLeadsRouter(reader).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, userID, reader.gotUserID)
	assert.Equal(t, analytics.PeriodMonth, reader.gotFilter.Period)
	assert.Equal(t, "qualified", reader.gotFilter.Status)
	assert.Equal(t, "telegram", reader.gotFilter.Channel)
	assert.Equal(t, 20, reader.gotFilter.Limit)

	var got struct {
		Leads []struct {
			ID             string `json:"id"`
			ContactName    string `json:"contact_name"`
			Channel        string `json:"channel"`
			Status         string `json:"status"`
			Score          *int   `json:"score"`
			ScoreReason    string `json:"score_reason"`
			LastActivityAt string `json:"last_activity_at"`
			QualifiedAt    string `json:"qualified_at"`
		} `json:"leads"`
		TotalMatching int `json:"total_matching"`
		LimitApplied  int `json:"limit_applied"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	require.Len(t, got.Leads, 1)
	assert.Equal(t, leadID.String(), got.Leads[0].ID)
	assert.Equal(t, "Acme Corp", got.Leads[0].ContactName)
	require.NotNil(t, got.Leads[0].Score)
	assert.Equal(t, 87, *got.Leads[0].Score)
	assert.Equal(t, 45, got.TotalMatching)
	assert.Equal(t, 20, got.LimitApplied)
}

func TestHandler_GetHotLeads_Defaults(t *testing.T) {
	reader := &stubHotLeadsReader{dto: &analytics.HotLeadsDTO{Leads: []analytics.HotLeadDTO{}, LimitApplied: 20}}
	w := httptest.NewRecorder()
	hotLeadsRouter(reader).ServeHTTP(w, newAuthedRequest(t, "/api/analytics/hot-leads", uuid.New()))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, analytics.PeriodAll, reader.gotFilter.Period, "missing period defaults to all")
	assert.Equal(t, analytics.FilterAny, reader.gotFilter.Status, "missing status defaults to any")
	assert.Equal(t, analytics.FilterAny, reader.gotFilter.Channel)
	assert.Equal(t, 20, reader.gotFilter.Limit, "missing limit defaults to 20")
}

func TestHandler_GetHotLeads_ClampsLimit(t *testing.T) {
	reader := &stubHotLeadsReader{dto: &analytics.HotLeadsDTO{Leads: []analytics.HotLeadDTO{}}}
	cases := map[string]int{"0": 20, "-5": 20, "9999": 100, "abc": 20, "50": 50}
	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			w := httptest.NewRecorder()
			hotLeadsRouter(reader).ServeHTTP(w, newAuthedRequest(t, "/api/analytics/hot-leads?limit="+raw, uuid.New()))
			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, want, reader.gotFilter.Limit)
		})
	}
}

func TestHandler_GetHotLeads_InvalidFilters(t *testing.T) {
	reader := &stubHotLeadsReader{dto: &analytics.HotLeadsDTO{}}
	for _, target := range []string{
		"/api/analytics/hot-leads?status=lost",
		"/api/analytics/hot-leads?channel=sms",
		"/api/analytics/hot-leads?period=year",
	} {
		w := httptest.NewRecorder()
		hotLeadsRouter(reader).ServeHTTP(w, newAuthedRequest(t, target, uuid.New()))
		assert.Equal(t, http.StatusBadRequest, w.Code, target)
	}
	assert.Equal(t, 0, reader.calls, "invalid filter must not reach the reader")
}

func TestHandler_GetHotLeads_Unauthenticated(t *testing.T) {
	reader := &stubHotLeadsReader{dto: &analytics.HotLeadsDTO{}}
	w := httptest.NewRecorder()
	// No auth context on the request.
	hotLeadsRouter(reader).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/analytics/hot-leads", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestParseStatusFilter(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "any", false},
		{"any", "any", false},
		{"new", "new", false},
		{"qualified", "qualified", false},
		{"in_conversation", "in_conversation", false},
		{"followup", "followup", false},
		{"closed", "closed", false},
		{"lost", "", true},      // not in this schema's lead_status enum
		{"garbage", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := analytics.ParseStatusFilter(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, errors.Is(err, analytics.ErrInvalidStatusFilter))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseChannelFilter(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "any", false},
		{"any", "any", false},
		{"telegram", "telegram", false},
		{"email", "email", false},
		{"sms", "", true},
		{"garbage", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := analytics.ParseChannelFilter(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, errors.Is(err, analytics.ErrInvalidChannelFilter))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
