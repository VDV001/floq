package enrichment_test

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/enrichment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTMLExtractor_FullPage(t *testing.T) {
	html := `<!DOCTYPE html><html><head>
		<title>  Acme LLC — Widgets  </title>
		<meta name="description" content="We make the best widgets in Russia.">
	</head><body>
		<a href="mailto:info@acme.ru">write us</a>
		Sales: sales@acme.ru or noreply@acme.ru
		<a href="tel:+7 (495) 123-45-67">call</a>
		<a href="https://t.me/acme_co">telegram</a>
		<a href="https://vk.com/acme">vk</a>
		<a href="https://www.linkedin.com/company/acme">li</a>
	</body></html>`

	ext := enrichment.NewHTMLExtractor()
	p, err := ext.Extract(context.Background(), html)
	require.NoError(t, err)

	assert.Equal(t, "Acme LLC — Widgets", p.Title)
	assert.Equal(t, "We make the best widgets in Russia.", p.Description)
	assert.Contains(t, p.Emails, "info@acme.ru")
	assert.Contains(t, p.Emails, "sales@acme.ru")
	assert.NotContains(t, p.Emails, "noreply@acme.ru", "junk addresses are filtered")
	assert.Contains(t, p.Phones, "+74951234567")
	assert.Contains(t, p.Socials, "https://t.me/acme_co")
	assert.Contains(t, p.Socials, "https://vk.com/acme")
	assert.Contains(t, p.Socials, "https://www.linkedin.com/company/acme")
}

func TestHTMLExtractor_MetaDescriptionReversedAttrs(t *testing.T) {
	html := `<meta content="Reversed attr order works." name="description">`
	p, err := enrichment.NewHTMLExtractor().Extract(context.Background(), html)
	require.NoError(t, err)
	assert.Equal(t, "Reversed attr order works.", p.Description)
}

func TestHTMLExtractor_OGDescriptionFallback(t *testing.T) {
	html := `<meta property="og:description" content="OG description fallback.">`
	p, err := enrichment.NewHTMLExtractor().Extract(context.Background(), html)
	require.NoError(t, err)
	assert.Equal(t, "OG description fallback.", p.Description)
}

func TestHTMLExtractor_DedupesAndSorts(t *testing.T) {
	html := `info@acme.ru info@acme.ru INFO@acme.ru
		<a href="https://t.me/acme">a</a><a href="https://t.me/acme">b</a>`
	p, err := enrichment.NewHTMLExtractor().Extract(context.Background(), html)
	require.NoError(t, err)
	assert.Equal(t, []string{"info@acme.ru"}, p.Emails, "case-insensitive dedup")
	assert.Equal(t, []string{"https://t.me/acme"}, p.Socials)
}

func TestHTMLExtractor_EmptyPage(t *testing.T) {
	p, err := enrichment.NewHTMLExtractor().Extract(context.Background(), "<html></html>")
	require.NoError(t, err)
	assert.True(t, p.IsEmpty(), "no signals → empty profile, no error")
}

func TestHTMLExtractor_GarbageInput(t *testing.T) {
	p, err := enrichment.NewHTMLExtractor().Extract(context.Background(), "\x00\xff not html at all")
	require.NoError(t, err)
	assert.True(t, p.IsEmpty())
}
