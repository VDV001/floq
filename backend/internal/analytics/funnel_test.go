package analytics_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daniil/floq/internal/analytics"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeBucketStep(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"valid 10", 10, 10},
		{"valid 20", 20, 20},
		{"valid 50", 50, 50},
		{"valid 100", 100, 100},
		{"zero falls back", 0, 10},
		{"negative falls back", -10, 10},
		{"not multiple of 10 falls back", 15, 10},
		{"too small falls back", 5, 10},
		{"too large falls back", 110, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, analytics.NormalizeBucketStep(tt.in))
		})
	}
}

// stubFunnel is a hand-rolled FunnelReader fake recording its inputs.
type stubFunnel struct {
	dist      *analytics.QualificationFunnelDTO
	conv      *analytics.SequenceConversionDTO
	gotUserID uuid.UUID
	gotStep   int
	gotPeriod analytics.Period
}

func (s *stubFunnel) GetQualificationDistribution(_ context.Context, userID uuid.UUID, step int, period analytics.Period) (*analytics.QualificationFunnelDTO, error) {
	s.gotUserID = userID
	s.gotStep = step
	s.gotPeriod = period
	return s.dist, nil
}

func (s *stubFunnel) GetSequenceConversion(_ context.Context, userID uuid.UUID, period analytics.Period) (*analytics.SequenceConversionDTO, error) {
	s.gotUserID = userID
	s.gotPeriod = period
	return s.conv, nil
}

func TestUseCase_GetQualificationDistribution_PassesConfiguredStep(t *testing.T) {
	stub := &stubFunnel{dist: &analytics.QualificationFunnelDTO{Step: 20}}
	uc := analytics.NewUseCase(nil, nil,
		analytics.WithFunnelReader(stub),
		analytics.WithScoreBucketStep(20))

	userID := uuid.New()
	_, err := uc.GetQualificationDistribution(context.Background(), userID, analytics.PeriodAll)
	require.NoError(t, err)
	assert.Equal(t, userID, stub.gotUserID)
	assert.Equal(t, 20, stub.gotStep, "usecase forwards the configured bucket step")
}

func TestUseCase_GetQualificationDistribution_DefaultStepIsTen(t *testing.T) {
	stub := &stubFunnel{dist: &analytics.QualificationFunnelDTO{}}
	uc := analytics.NewUseCase(nil, nil, analytics.WithFunnelReader(stub))

	_, err := uc.GetQualificationDistribution(context.Background(), uuid.New(), analytics.PeriodAll)
	require.NoError(t, err)
	assert.Equal(t, 10, stub.gotStep, "default bucket step is 10")
}

func TestHandler_GetQualificationDistribution_ReturnsBuckets(t *testing.T) {
	userID := uuid.New()
	stub := &stubFunnel{dist: &analytics.QualificationFunnelDTO{
		Step:  50,
		Total: 5,
		Buckets: []analytics.QualBucketDTO{
			{Lo: 0, Hi: 49, Label: "0–49", Count: 5},
			{Lo: 50, Hi: 100, Label: "50–100", Count: 0},
		},
	}}

	r := chi.NewRouter()
	analytics.RegisterRoutes(r, analytics.NewUseCase(nil, nil,
		analytics.WithFunnelReader(stub),
		analytics.WithScoreBucketStep(50)))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newAuthedRequest(t, "/api/analytics/qualification-distribution", userID))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, userID, stub.gotUserID)

	var got struct {
		Step    int              `json:"step"`
		Total   int              `json:"total"`
		Buckets []map[string]any `json:"buckets"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, 50, got.Step)
	assert.Equal(t, 5, got.Total)
	require.Len(t, got.Buckets, 2)
	assert.Equal(t, "0–49", got.Buckets[0]["label"])
	assert.EqualValues(t, 5, got.Buckets[0]["count"])
}

func TestHandler_GetSequenceConversion_ReturnsSteps(t *testing.T) {
	userID := uuid.New()
	seqID := uuid.New()
	stub := &stubFunnel{conv: &analytics.SequenceConversionDTO{
		Steps: []analytics.SequenceStepConversionDTO{{
			SequenceID: seqID, SequenceName: "Warm intro", StepOrder: 1,
			Entered: 3, Replied: 2, Advanced: 1,
			ReplyRate: 2.0 / 3.0, AdvanceRate: 1.0 / 3.0,
		}},
	}}

	r := chi.NewRouter()
	analytics.RegisterRoutes(r, analytics.NewUseCase(nil, nil, analytics.WithFunnelReader(stub)))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newAuthedRequest(t, "/api/analytics/sequence-conversion", userID))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, userID, stub.gotUserID)

	var got struct {
		Steps []map[string]any `json:"steps"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	require.Len(t, got.Steps, 1)
	assert.Equal(t, seqID.String(), got.Steps[0]["sequence_id"])
	assert.Equal(t, "Warm intro", got.Steps[0]["sequence_name"])
	assert.EqualValues(t, 3, got.Steps[0]["entered"])
	assert.EqualValues(t, 2, got.Steps[0]["replied"])
	assert.InDelta(t, 2.0/3.0, got.Steps[0]["reply_rate"], 0.001)
}
