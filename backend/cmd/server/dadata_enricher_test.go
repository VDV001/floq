package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func TestDaDataEnricher_FindByName_ExactMatchWins(t *testing.T) {
	stub := newDadataStub(t)
	stub.suggestions = []map[string]any{
		partySuggestion("Акме", "7707083893", "1027700132195", "Москва", "62.01", "ACTIVE"),
		partySuggestion("Акме Сервис", "7708503727", "1037739877295", "Москва", "62.02", "ACTIVE"),
	}
	enr := newDaDataEnricher(stub.srv.Client(), "k", stub.srv.URL)

	legal, found, err := enr.Enrich(context.Background(), enrichment.EnrichQuery{CompanyName: "  акме "})
	require.NoError(t, err)
	require.True(t, found, "an exact (normalized) name match disambiguates")
	assert.Equal(t, "7707083893", legal.INN)
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
