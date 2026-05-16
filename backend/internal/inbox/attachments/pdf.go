package attachments

import (
	"bytes"
	"strings"

	"github.com/ledongthuc/pdf"
)

// extractPDFText reads `data` as a PDF document and returns the
// concatenated text content of its pages plus the page count. The
// MaxPDFPages limit is checked after parsing the catalog (cheap) but
// before iterating the page contents (expensive). An empty result
// surfaces as ErrNoTextLayer so the caller can choose between
// SkipNoTextLayer and a vision fallback (out of scope here — image-only
// attachments go through the image path).
func extractPDFText(data []byte) (text string, pages int, err error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", 0, err
	}

	pages = reader.NumPage()
	if pages > MaxPDFPages {
		return "", pages, ErrTooManyPages
	}

	var sb strings.Builder
	for i := 1; i <= pages; i++ {
		p := reader.Page(i)
		if p.V.IsNull() {
			continue
		}
		pageText, err := p.GetPlainText(nil)
		if err != nil {
			// One bad page shouldn't kill the whole extract — skip and
			// try the next. The caller still sees the joined text from
			// pages that did parse.
			continue
		}
		sb.WriteString(pageText)
		sb.WriteByte('\n')
	}

	text = strings.TrimSpace(sb.String())
	if text == "" {
		return "", pages, ErrNoTextLayer
	}
	return text, pages, nil
}
