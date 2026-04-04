package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TwoGISResult represents a single company from 2GIS search results.
type TwoGISResult struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	Phone    string `json:"phone"`
	Category string `json:"category"`
	Website  string `json:"website"`
	City     string `json:"city"`
}

// Known region IDs for major Russian cities in 2GIS.
var cityIDs = map[string]int{
	"москва":          32,
	"санкт-петербург": 2,
	"новосибирск":     1,
	"екатеринбург":    7,
	"казань":          12,
	"нижний новгород": 11,
	"красноярск":      81,
	"челябинск":       3,
	"самара":          6,
	"уфа":             5,
	"ростов-на-дону":  60,
	"краснодар":       44,
	"омск":            4,
	"воронеж":         26,
	"пермь":           8,
	"волгоград":       36,
	"тюмень":          9,
	"томск":           92,
	"барнаул":         110,
	"иркутск":         63,
}

// TwoGISClient wraps 2GIS API calls with a configurable API key.
type TwoGISClient struct {
	APIKey string
}

// NewTwoGISClient creates a new client. If apiKey is empty, a default public widget key is used.
func NewTwoGISClient(apiKey string) *TwoGISClient {
	if apiKey == "" {
		apiKey = "rurbbn3446"
	}
	return &TwoGISClient{APIKey: apiKey}
}

// Search searches for companies on 2GIS by query and city name.
func (c *TwoGISClient) Search(ctx context.Context, query, city string) ([]TwoGISResult, error) {
	cityLower := strings.ToLower(city)
	regionID, ok := cityIDs[cityLower]
	if !ok {
		regionID = 32 // default to Moscow
	}

	apiURL := fmt.Sprintf(
		"https://catalog.api.2gis.ru/3.0/items?q=%s&region_id=%d&page=1&page_size=50&fields=items.contact_groups,items.address&key=%s",
		url.QueryEscape(query), regionID, c.APIKey,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("2gis: create request: %w", err)
	}
	req.Header.Set("User-Agent", "Floq/1.0")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("2gis: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("2gis: status %d", resp.StatusCode)
	}

	var apiResp struct {
		Result struct {
			Items []struct {
				Name        string `json:"name"`
				AddressName string `json:"address_name"`
				FullName    string `json:"full_name"`
				ContactGroups []struct {
					Contacts []struct {
						Type  string `json:"type"`
						Value string `json:"value"`
					} `json:"contacts"`
				} `json:"contact_groups"`
				Rubrics []struct {
					Name string `json:"name"`
				} `json:"rubrics"`
			} `json:"items"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("2gis: decode response: %w", err)
	}

	var results []TwoGISResult
	for _, item := range apiResp.Result.Items {
		r := TwoGISResult{
			Name:    item.Name,
			Address: item.AddressName,
			City:    city,
		}
		if len(item.Rubrics) > 0 {
			r.Category = item.Rubrics[0].Name
		}
		for _, cg := range item.ContactGroups {
			for _, c := range cg.Contacts {
				switch c.Type {
				case "phone":
					if r.Phone == "" {
						r.Phone = c.Value
					}
				case "website":
					if r.Website == "" {
						r.Website = c.Value
					}
				}
			}
		}
		results = append(results, r)
	}

	return results, nil
}
