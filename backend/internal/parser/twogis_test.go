package parser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCityIDs_KnownCities(t *testing.T) {
	tests := []struct {
		city string
		id   int
	}{
		{"москва", 32},
		{"санкт-петербург", 2},
		{"новосибирск", 1},
		{"екатеринбург", 7},
		{"казань", 12},
		{"нижний новгород", 11},
		{"красноярск", 81},
		{"челябинск", 3},
		{"самара", 6},
		{"уфа", 5},
		{"ростов-на-дону", 60},
		{"краснодар", 44},
		{"омск", 4},
		{"воронеж", 26},
		{"пермь", 8},
		{"волгоград", 36},
		{"тюмень", 9},
		{"томск", 92},
		{"барнаул", 110},
		{"иркутск", 63},
	}
	for _, tc := range tests {
		id, ok := cityIDs[tc.city]
		assert.True(t, ok, "city not found: %s", tc.city)
		assert.Equal(t, tc.id, id, "city: %s", tc.city)
	}
}

func TestCityIDs_UnknownCity(t *testing.T) {
	_, ok := cityIDs["владивосток"]
	assert.False(t, ok)
}

func TestCityIDs_Count(t *testing.T) {
	assert.Equal(t, 20, len(cityIDs))
}

func TestNewTwoGISClient(t *testing.T) {
	c := NewTwoGISClient("test-key", nil)
	assert.Equal(t, "test-key", c.APIKey)
}

func TestSearch_HappyPath(t *testing.T) {
	resp := map[string]any{
		"result": map[string]any{
			"items": []any{
				map[string]any{
					"name":         "Acme Corp",
					"address_name": "ул. Ленина, 1",
					"rubrics":      []any{map[string]any{"name": "IT"}},
					"contact_groups": []any{
						map[string]any{
							"contacts": []any{
								map[string]any{"type": "phone", "value": "+7999"},
								map[string]any{"type": "phone", "value": "+7888"}, // second phone ignored
								map[string]any{"type": "website", "value": "https://acme.com"},
								map[string]any{"type": "website", "value": "https://acme2.com"}, // second website ignored
								map[string]any{"type": "email", "value": "a@b.com"},              // unknown type
							},
						},
					},
				},
				map[string]any{
					"name":         "NoContacts",
					"address_name": "ул. Мира, 2",
				},
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "q=restaurants")
		assert.Contains(t, r.URL.RawQuery, "region_id=32")
		assert.Contains(t, r.URL.RawQuery, "key=test-key")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := &TwoGISClient{APIKey: "test-key", baseURL: srv.URL}
	results, err := c.Search(context.Background(), "restaurants", "Москва")
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, "Acme Corp", results[0].Name)
	assert.Equal(t, "ул. Ленина, 1", results[0].Address)
	assert.Equal(t, "IT", results[0].Category)
	assert.Equal(t, "+7999", results[0].Phone)
	assert.Equal(t, "https://acme.com", results[0].Website)
	assert.Equal(t, "Москва", results[0].City)

	assert.Equal(t, "NoContacts", results[1].Name)
	assert.Empty(t, results[1].Phone)
	assert.Empty(t, results[1].Website)
	assert.Empty(t, results[1].Category)
}

func TestSearch_UnknownCity_DefaultsMoscow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "region_id=32")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"items": []any{}}})
	}))
	defer srv.Close()

	c := &TwoGISClient{APIKey: "k", baseURL: srv.URL}
	results, err := c.Search(context.Background(), "test", "Владивосток")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearch_KnownCity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "region_id=2")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"items": []any{}}})
	}))
	defer srv.Close()

	c := &TwoGISClient{APIKey: "k", baseURL: srv.URL}
	_, err := c.Search(context.Background(), "test", "Санкт-Петербург")
	require.NoError(t, err)
}

func TestSearch_Non200Status(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := &TwoGISClient{APIKey: "k", baseURL: srv.URL}
	_, err := c.Search(context.Background(), "test", "Москва")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 403")
}

func TestSearch_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := &TwoGISClient{APIKey: "k", baseURL: srv.URL}
	_, err := c.Search(context.Background(), "test", "Москва")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

func TestSearch_CancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &TwoGISClient{APIKey: "k", baseURL: srv.URL}
	_, err := c.Search(ctx, "test", "Москва")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "request failed")
}
