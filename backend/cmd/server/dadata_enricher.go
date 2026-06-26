package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/daniil/floq/internal/enrichment"
	"github.com/daniil/floq/internal/enrichment/domain"
)

// daDataBaseURL is the DaData party suggestions/lookup base. A constant,
// trusted host — the request URL is never derived from untrusted input (unlike
// the website fetcher), so there is no SSRF surface and a standard http client
// is sufficient.
const daDataBaseURL = "https://suggestions.dadata.ru/suggestions/api/4_1/rs"

// daDataEnricher implements enrichment.Enricher over the official DaData party
// API. INN present → precise findById lookup; otherwise a fuzzy name suggest
// with a confidence gate (single hit, or an exact normalized name match) — it
// returns a miss rather than guess when the match is ambiguous.
type daDataEnricher struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
}

var _ enrichment.Enricher = (*daDataEnricher)(nil)

func newDaDataEnricher(client *http.Client, apiKey, baseURL string) *daDataEnricher {
	if baseURL == "" {
		baseURL = daDataBaseURL
	}
	return &daDataEnricher{
		httpClient: client,
		apiKey:     apiKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

// ddResponse is the slice of the DaData party response we consume.
type ddResponse struct {
	Suggestions []ddSuggestion `json:"suggestions"`
}

type ddSuggestion struct {
	Value string `json:"value"`
	Data  struct {
		INN   string `json:"inn"`
		OGRN  string `json:"ogrn"`
		OKVED string `json:"okved"`
		Name  struct {
			FullWithOpf string `json:"full_with_opf"`
		} `json:"name"`
		Address struct {
			Value string `json:"value"`
		} `json:"address"`
		State struct {
			Status string `json:"status"`
		} `json:"state"`
	} `json:"data"`
}

func (d *daDataEnricher) Enrich(ctx context.Context, q enrichment.EnrichQuery) (domain.LegalDetails, bool, error) {
	switch {
	case q.INN != "":
		resp, err := d.post(ctx, "/findById/party", q.INN, 1)
		if err != nil {
			return domain.LegalDetails{}, false, err
		}
		if len(resp.Suggestions) == 0 {
			return domain.LegalDetails{}, false, nil
		}
		return mapSuggestion(resp.Suggestions[0]), true, nil

	case q.CompanyName != "":
		resp, err := d.post(ctx, "/suggest/party", q.CompanyName, 2)
		if err != nil {
			return domain.LegalDetails{}, false, err
		}
		s, ok := pickConfident(resp.Suggestions, q.CompanyName)
		if !ok {
			return domain.LegalDetails{}, false, nil
		}
		return mapSuggestion(s), true, nil

	default:
		return domain.LegalDetails{}, false, nil // no signal → miss, no API call
	}
}

// pickConfident returns a single confident match: the sole result, or — when
// several fuzzy results come back — the first only if its name matches the
// query exactly (normalized). Otherwise no match (don't guess).
func pickConfident(sg []ddSuggestion, query string) (ddSuggestion, bool) {
	switch len(sg) {
	case 0:
		return ddSuggestion{}, false
	case 1:
		return sg[0], true
	default:
		if normalizeName(sg[0].Value) == normalizeName(query) {
			return sg[0], true
		}
		return ddSuggestion{}, false
	}
}

func normalizeName(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// mapSuggestion converts a DaData suggestion to LegalDetails. INN/OGRN are
// validated through the domain VOs and only set when they pass their checksum
// (defensive — an upstream change cannot inject a malformed id).
func mapSuggestion(s ddSuggestion) domain.LegalDetails {
	d := domain.LegalDetails{
		FullName: s.Data.Name.FullWithOpf,
		Address:  s.Data.Address.Value,
		OKVED:    s.Data.OKVED,
		Status:   s.Data.State.Status,
	}
	if inn, err := domain.NewINN(s.Data.INN); err == nil {
		d.INN = inn.String()
	}
	if ogrn, err := domain.NewOGRN(s.Data.OGRN); err == nil {
		d.OGRN = ogrn.String()
	}
	return d
}

func (d *daDataEnricher) post(ctx context.Context, endpoint, query string, count int) (ddResponse, error) {
	payload, _ := json.Marshal(map[string]any{"query": query, "count": count})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+endpoint, bytes.NewReader(payload))
	if err != nil {
		return ddResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Token "+d.apiKey)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return ddResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ddResponse{}, fmt.Errorf("dadata: unexpected status %d", resp.StatusCode)
	}
	var out ddResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ddResponse{}, fmt.Errorf("dadata: decode response: %w", err)
	}
	return out, nil
}
