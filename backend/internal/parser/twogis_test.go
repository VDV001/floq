package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	c := NewTwoGISClient("test-key")
	assert.Equal(t, "test-key", c.APIKey)
}
