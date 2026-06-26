package enrichment_test

import (
	"context"
	"errors"
	"testing"

	"github.com/daniil/floq/internal/enrichment"
	"github.com/daniil/floq/internal/enrichment/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCompleter is a stub Completer: it returns a canned response (or error)
// and records the prompts it was called with.
type fakeCompleter struct {
	resp    string
	err     error
	gotSys  string
	gotUser string
	calls   int
}

func (f *fakeCompleter) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	f.calls++
	f.gotSys = systemPrompt
	f.gotUser = userPrompt
	return f.resp, f.err
}

func TestLLMExtractor_Extract(t *testing.T) {
	cases := []struct {
		name         string
		resp         string
		wantIndustry string
		wantSize     domain.CompanySize
	}{
		{
			name:         "clean json",
			resp:         `{"industry":"Fintech","company_size":"medium"}`,
			wantIndustry: "fintech",
			wantSize:     domain.CompanySizeMedium,
		},
		{
			name:         "markdown fenced json",
			resp:         "```json\n{\"industry\":\"Logistics\",\"company_size\":\"large\"}\n```",
			wantIndustry: "logistics",
			wantSize:     domain.CompanySizeLarge,
		},
		{
			name:         "json with surrounding prose",
			resp:         `Here you go: {"industry":"E-commerce","company_size":"small"} hope it helps`,
			wantIndustry: "e-commerce",
			wantSize:     domain.CompanySizeSmall,
		},
		{
			name:         "invalid size falls back to unknown",
			resp:         `{"industry":"SaaS","company_size":"gigantic"}`,
			wantIndustry: "saas",
			wantSize:     domain.CompanySizeUnknown,
		},
		{
			name:         "size case-normalized",
			resp:         `{"industry":"Retail","company_size":"  ENTERPRISE "}`,
			wantIndustry: "retail",
			wantSize:     domain.CompanySizeEnterprise,
		},
		{
			name:         "empty fields stay empty",
			resp:         `{"industry":"","company_size":""}`,
			wantIndustry: "",
			wantSize:     domain.CompanySizeUnknown,
		},
		{
			// A model that appends a stray brace or extra object after the
			// first must not break parsing (last-"}" scanning would).
			name:         "trailing junk after the object",
			resp:         `{"industry":"Media","company_size":"small"}} extra`,
			wantIndustry: "media",
			wantSize:     domain.CompanySizeSmall,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fc := &fakeCompleter{resp: c.resp}
			ex := enrichment.NewLLMExtractor(fc)

			got, err := ex.Extract(context.Background(), "<html>Acme</html>")
			require.NoError(t, err)
			assert.Equal(t, c.wantIndustry, got.Industry)
			assert.Equal(t, c.wantSize, got.CompanySize)
			// LLMExtractor only fills the Phase-2 fields; it never invents contacts.
			assert.Empty(t, got.Title)
			assert.Empty(t, got.Emails)
			assert.Equal(t, "<html>Acme</html>", fc.gotUser, "page is sent as the user prompt")
			assert.NotEmpty(t, fc.gotSys, "a system prompt frames the extraction")
		})
	}
}

func TestLLMExtractor_PropagatesCompleterError(t *testing.T) {
	sentinel := errors.New("provider down")
	ex := enrichment.NewLLMExtractor(&fakeCompleter{err: sentinel})

	_, err := ex.Extract(context.Background(), "page")
	assert.ErrorIs(t, err, sentinel)
}

func TestLLMExtractor_ErrorsOnUnparseableResponse(t *testing.T) {
	ex := enrichment.NewLLMExtractor(&fakeCompleter{resp: "not json at all"})

	_, err := ex.Extract(context.Background(), "page")
	assert.Error(t, err, "a response with no JSON object is an extraction error")
}

// LLMExtractor must satisfy the Extractor port so it can slot into a chain.
func TestLLMExtractor_SatisfiesExtractorPort(t *testing.T) {
	var _ enrichment.Extractor = enrichment.NewLLMExtractor(&fakeCompleter{})
}
