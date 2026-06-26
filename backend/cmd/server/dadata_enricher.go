package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/daniil/floq/internal/enrichment"
	"github.com/daniil/floq/internal/enrichment/domain"
)

// rateLimiter bounds DaData egress to protect the shared daily quota. Matches
// the ratelimit.Limiter shape structurally (declared locally; injected from
// the composition root). nil = unlimited.
type rateLimiter interface {
	Allow(ctx context.Context, key string) (bool, time.Duration, error)
}

// dadataRateLimitKey is the single global bucket key — the quota is per API
// key (global), not per company.
const dadataRateLimitKey = "dadata"

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
	limiter    rateLimiter         // optional global egress cap; nil = unlimited
	observe    func(result string) // metrics hook; no-op unless wired from the composition root
}

var _ enrichment.Enricher = (*daDataEnricher)(nil)

// registryOutcome classifies one registry enrichment attempt for the metrics
// observer; its values are the `result` label of
// enrichment_registry_requests_total. outcomeNoSignal is the sentinel for
// "nothing to look up" — it is never observed, because no attempt was made.
type registryOutcome string

const (
	outcomeNoSignal    registryOutcome = ""
	outcomeRateLimited registryOutcome = "rate_limited"
	outcomeHit         registryOutcome = "hit"
	outcomeMiss        registryOutcome = "miss"
	outcomeError       registryOutcome = "error"
)

func newDaDataEnricher(client *http.Client, apiKey, baseURL string, limiter ...rateLimiter) *daDataEnricher {
	if baseURL == "" {
		baseURL = daDataBaseURL
	}
	var lim rateLimiter
	if len(limiter) > 0 {
		lim = limiter[0]
	}
	return &daDataEnricher{
		httpClient: client,
		apiKey:     apiKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
		limiter:    lim,
		observe:    func(string) {}, // no-op default; composition root wires metrics
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
	legal, outcome, err := d.lookup(ctx, q)
	// A no-signal early return is not a registry attempt — nothing was looked
	// up — so it is deliberately not counted. Every other path (a real lookup,
	// an error, or a throttled skip) is exactly one observed outcome.
	if outcome != outcomeNoSignal {
		d.observe(string(outcome))
	}
	return legal, outcome == outcomeHit, err
}

// lookup runs the registry query and classifies the result into a single
// registryOutcome, so Enrich has one place to map to the port's bool and one
// place to observe metrics (no scattered Inc() calls to forget on a new
// return).
func (d *daDataEnricher) lookup(ctx context.Context, q enrichment.EnrichQuery) (domain.LegalDetails, registryOutcome, error) {
	if q.INN == "" && q.CompanyName == "" {
		return domain.LegalDetails{}, outcomeNoSignal, nil // no signal → no API call, no budget spent
	}
	// Global egress cap (protects the shared daily quota). Over budget → skip
	// (a miss), never an error — registry is best-effort.
	if d.limiter != nil {
		allowed, _, err := d.limiter.Allow(ctx, dadataRateLimitKey)
		if err != nil || !allowed {
			return domain.LegalDetails{}, outcomeRateLimited, nil
		}
	}
	switch {
	case q.INN != "":
		resp, err := d.post(ctx, "/findById/party", q.INN, 1)
		if err != nil {
			return domain.LegalDetails{}, outcomeError, err
		}
		if len(resp.Suggestions) == 0 {
			return domain.LegalDetails{}, outcomeMiss, nil
		}
		return mapSuggestion(resp.Suggestions[0]), outcomeHit, nil

	case q.CompanyName != "":
		// Ask for 2 so we can detect ambiguity: anything but a single unique
		// hit is treated as "not confidently identified".
		resp, err := d.post(ctx, "/suggest/party", q.CompanyName, 2)
		if err != nil {
			return domain.LegalDetails{}, outcomeError, err
		}
		s, ok := pickConfident(resp.Suggestions)
		if !ok {
			return domain.LegalDetails{}, outcomeMiss, nil
		}
		return mapSuggestion(s), outcomeHit, nil

	default:
		return domain.LegalDetails{}, outcomeNoSignal, nil // no signal → no API call
	}
}

// pickConfident accepts a fuzzy name result only when it is a single unique
// hit. Any set of 2+ is ambiguous — a same-named match among them does NOT
// identify the company (many distinct entities share a name), so we return no
// match rather than write a wrong ИНН/ОГРН. The precise INN path covers the
// high-confidence case.
func pickConfident(sg []ddSuggestion) (ddSuggestion, bool) {
	if len(sg) == 1 {
		return sg[0], true
	}
	return ddSuggestion{}, false
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
