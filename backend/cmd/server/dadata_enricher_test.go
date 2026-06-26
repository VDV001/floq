package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/daniil/floq/internal/enrichment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dadataStub spins an httptest server mimicking DaData's suggest/findById
// endpoints. It records the last request and returns canned suggestions.
type dadataStub struct {
	srv         *httptest.Server
	lastPath    string
	lastAuth    string
	lastQuery   string
	suggestions []map[string]any
}

func newDadataStub(t *testing.T) *dadataStub {
	t.Helper()
	s := &dadataStub{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.lastPath = r.URL.Path
		s.lastAuth = r.Header.Get("Authorization")
		var body struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.lastQuery = body.Query
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"suggestions": s.suggestions})
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func partySuggestion(name, inn, ogrn, addr, okved, status string) map[string]any {
	return map[string]any{
		"value": name,
		"data": map[string]any{
			"inn":     inn,
			"ogrn":    ogrn,
			"okved":   okved,
			"name":    map[string]any{"full_with_opf": name},
			"address": map[string]any{"value": addr},
			"state":   map[string]any{"status": status},
		},
	}
}

func TestDaDataEnricher_FindByINN(t *testing.T) {
	stub := newDadataStub(t)
	stub.suggestions = []map[string]any{
		partySuggestion("ПАО СБЕРБАНК", "7707083893", "1027700132195", "г Москва", "64.19", "ACTIVE"),
	}
	enr := newDaDataEnricher(stub.srv.Client(), "secret-key", stub.srv.URL)

	legal, found, err := enr.Enrich(context.Background(), enrichment.EnrichQuery{INN: "7707083893", CompanyName: "Acme"})
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "7707083893", legal.INN)
	assert.Equal(t, "1027700132195", legal.OGRN)
	assert.Equal(t, "г Москва", legal.Address)
	assert.Equal(t, "64.19", legal.OKVED)
	assert.Equal(t, "ПАО СБЕРБАНК", legal.FullName)
	assert.Equal(t, "ACTIVE", legal.Status)
	// INN present → precise findById endpoint, with Token auth.
	assert.Contains(t, stub.lastPath, "findById")
	assert.Equal(t, "Token secret-key", stub.lastAuth)
	assert.Equal(t, "7707083893", stub.lastQuery)
}

func TestDaDataEnricher_FindByName_SingleHit(t *testing.T) {
	stub := newDadataStub(t)
	stub.suggestions = []map[string]any{
		partySuggestion("ООО Акме", "7707083893", "1027700132195", "Москва", "62.01", "ACTIVE"),
	}
	enr := newDaDataEnricher(stub.srv.Client(), "k", stub.srv.URL)

	legal, found, err := enr.Enrich(context.Background(), enrichment.EnrichQuery{CompanyName: "Акме"})
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "7707083893", legal.INN)
	assert.Contains(t, stub.lastPath, "suggest")
}

func TestDaDataEnricher_FindByName_AmbiguousIsMiss(t *testing.T) {
	stub := newDadataStub(t)
	stub.suggestions = []map[string]any{
		partySuggestion("ООО Акме Плюс", "7707083893", "1027700132195", "Москва", "62.01", "ACTIVE"),
		partySuggestion("ООО Акме Сервис", "7708503727", "1037739877295", "Москва", "62.02", "ACTIVE"),
	}
	enr := newDaDataEnricher(stub.srv.Client(), "k", stub.srv.URL)

	_, found, err := enr.Enrich(context.Background(), enrichment.EnrichQuery{CompanyName: "Акме"})
	require.NoError(t, err)
	assert.False(t, found, "multiple fuzzy hits with no exact name match → skip, don't guess")
}

func TestDaDataEnricher_FindByName_ExactNameAmongAmbiguousIsMiss(t *testing.T) {
	// Many distinct legal entities share an identical name (dozens of «ООО
	// Ромашка»). An exact name match within an ambiguous result set does NOT
	// identify the right company, so the honest answer is a miss — never guess.
	stub := newDadataStub(t)
	stub.suggestions = []map[string]any{
		partySuggestion("Акме", "7707083893", "1027700132195", "Москва", "62.01", "ACTIVE"),
		partySuggestion("Акме", "7708503727", "1037739877295", "Москва", "62.02", "ACTIVE"),
	}
	enr := newDaDataEnricher(stub.srv.Client(), "k", stub.srv.URL)

	_, found, err := enr.Enrich(context.Background(), enrichment.EnrichQuery{CompanyName: "  акме "})
	require.NoError(t, err)
	assert.False(t, found, "ambiguous name → miss even if a result name matches exactly")
}

func TestDaDataEnricher_NoResults_Miss(t *testing.T) {
	stub := newDadataStub(t)
	stub.suggestions = nil
	enr := newDaDataEnricher(stub.srv.Client(), "k", stub.srv.URL)

	_, found, err := enr.Enrich(context.Background(), enrichment.EnrichQuery{CompanyName: "Nonexistent"})
	require.NoError(t, err)
	assert.False(t, found)
}

func TestDaDataEnricher_NoSignal_Miss(t *testing.T) {
	stub := newDadataStub(t)
	enr := newDaDataEnricher(stub.srv.Client(), "k", stub.srv.URL)

	_, found, err := enr.Enrich(context.Background(), enrichment.EnrichQuery{})
	require.NoError(t, err)
	assert.False(t, found, "no INN and no name → miss without calling the API")
	assert.Empty(t, stub.lastPath, "API not called when there is no signal")
}

func TestDaDataEnricher_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	enr := newDaDataEnricher(srv.Client(), "k", srv.URL)

	_, _, err := enr.Enrich(context.Background(), enrichment.EnrichQuery{INN: "7707083893"})
	assert.Error(t, err)
}

// denyLimiter always denies — stands in for an exhausted DaData budget.
type denyLimiter struct{ calls int }

func (l *denyLimiter) Allow(_ context.Context, _ string) (bool, time.Duration, error) {
	l.calls++
	return false, time.Minute, nil
}

func TestDaDataEnricher_RateLimited_SkipsWithoutCalling(t *testing.T) {
	stub := newDadataStub(t)
	stub.suggestions = []map[string]any{partySuggestion("X", "7707083893", "1027700132195", "A", "1", "ACTIVE")}
	lim := &denyLimiter{}
	enr := newDaDataEnricher(stub.srv.Client(), "k", stub.srv.URL, lim)

	_, found, err := enr.Enrich(context.Background(), enrichment.EnrichQuery{INN: "7707083893"})
	require.NoError(t, err, "over-budget is a skip, not an error")
	assert.False(t, found)
	assert.Equal(t, 1, lim.calls, "limiter consulted")
	assert.Empty(t, stub.lastPath, "no API call when over budget — protects the daily quota")
}

func TestDaDataEnricher_ObservesOutcomes(t *testing.T) {
	tests := []struct {
		name        string
		suggestions []map[string]any
		query       enrichment.EnrichQuery
		want        string
	}{
		{
			name:        "inn hit",
			suggestions: []map[string]any{partySuggestion("X", "7707083893", "1027700132195", "A", "1", "ACTIVE")},
			query:       enrichment.EnrichQuery{INN: "7707083893"},
			want:        "hit",
		},
		{
			name:        "name single hit",
			suggestions: []map[string]any{partySuggestion("Acme", "7707083893", "1027700132195", "A", "1", "ACTIVE")},
			query:       enrichment.EnrichQuery{CompanyName: "Acme"},
			want:        "hit",
		},
		{
			name:        "no results is a miss",
			suggestions: nil,
			query:       enrichment.EnrichQuery{CompanyName: "Nope"},
			want:        "miss",
		},
		{
			name: "ambiguous name is a miss",
			suggestions: []map[string]any{
				partySuggestion("Acme A", "7707083893", "1027700132195", "A", "1", "ACTIVE"),
				partySuggestion("Acme B", "7708503727", "1037739877295", "B", "2", "ACTIVE"),
			},
			query: enrichment.EnrichQuery{CompanyName: "Acme"},
			want:  "miss",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stub := newDadataStub(t)
			stub.suggestions = tc.suggestions
			enr := newDaDataEnricher(stub.srv.Client(), "k", stub.srv.URL)
			var got []string
			enr.observe = func(result string) { got = append(got, result) }

			_, _, err := enr.Enrich(context.Background(), tc.query)
			require.NoError(t, err)
			assert.Equal(t, []string{tc.want}, got)
		})
	}
}

func TestDaDataEnricher_ObservesErrorOutcome(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	enr := newDaDataEnricher(srv.Client(), "k", srv.URL)
	var got []string
	enr.observe = func(result string) { got = append(got, result) }

	_, _, err := enr.Enrich(context.Background(), enrichment.EnrichQuery{INN: "7707083893"})
	require.Error(t, err)
	assert.Equal(t, []string{"error"}, got, "an API/transport failure is its own outcome, not a miss")
}

func TestDaDataEnricher_ObservesRateLimitedOutcome(t *testing.T) {
	stub := newDadataStub(t)
	stub.suggestions = []map[string]any{partySuggestion("X", "7707083893", "1027700132195", "A", "1", "ACTIVE")}
	enr := newDaDataEnricher(stub.srv.Client(), "k", stub.srv.URL, &denyLimiter{})
	var got []string
	enr.observe = func(result string) { got = append(got, result) }

	_, found, err := enr.Enrich(context.Background(), enrichment.EnrichQuery{INN: "7707083893"})
	require.NoError(t, err)
	assert.False(t, found)
	assert.Equal(t, []string{"rate_limited"}, got, "a throttled skip is distinct from a miss — it signals quota pressure")
}

func TestDaDataEnricher_NoSignal_NotObserved(t *testing.T) {
	stub := newDadataStub(t)
	enr := newDaDataEnricher(stub.srv.Client(), "k", stub.srv.URL)
	var got []string
	enr.observe = func(result string) { got = append(got, result) }

	_, _, err := enr.Enrich(context.Background(), enrichment.EnrichQuery{})
	require.NoError(t, err)
	assert.Empty(t, got, "no INN/name → no registry attempt → nothing to count")
}

func TestDaDataEnricher_SatisfiesPort(t *testing.T) {
	var _ enrichment.Enricher = newDaDataEnricher(http.DefaultClient, "k", "")
}

// guard against accidental double-slash when joining base + path
func TestDaDataEnricher_BaseURLTrailingSlash(t *testing.T) {
	stub := newDadataStub(t)
	stub.suggestions = []map[string]any{partySuggestion("X", "7707083893", "1027700132195", "A", "1", "ACTIVE")}
	enr := newDaDataEnricher(stub.srv.Client(), "k", stub.srv.URL+"/")
	_, found, err := enr.Enrich(context.Background(), enrichment.EnrichQuery{INN: "7707083893"})
	require.NoError(t, err)
	assert.True(t, found)
	assert.False(t, strings.Contains(stub.lastPath, "//"), "no double slash in path")
}
