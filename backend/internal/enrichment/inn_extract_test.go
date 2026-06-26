package enrichment_test

import (
	"testing"

	"github.com/daniil/floq/internal/enrichment"
	"github.com/stretchr/testify/assert"
)

func TestExtractINN(t *testing.T) {
	cases := []struct {
		name string
		page string
		want string
	}{
		{"labeled with colon", "Контакты ИНН: 7707083893 КПП 770701001", "7707083893"},
		{"labeled with space", "ИНН 7707083893", "7707083893"},
		{"12-digit sole proprietor", "ИНН 500100732259", "500100732259"},
		{"lowercase label", "инн 7707083893", "7707083893"},
		{"bad checksum ignored", "ИНН 7707083890", ""},
		{"no label no match", "просто текст 7707083893 без метки", ""},
		{"no inn at all", "<html>Acme</html>", ""},
		{"label but garbage digits", "ИНН 12345", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, enrichment.ExtractINN(c.page))
		})
	}
}
