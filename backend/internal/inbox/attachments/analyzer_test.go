package attachments

import (
	"context"
	_ "embed"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed testdata/hello.pdf
var helloPDF []byte

// --- helpers ---

type mockVisionClient struct {
	resp string
	err  error
	last struct {
		mime   string
		prompt string
		bytes  int
	}
}

func (m *mockVisionClient) AnalyzeImage(_ context.Context, data []byte, mimeType, prompt string) (string, error) {
	m.last.mime = mimeType
	m.last.prompt = prompt
	m.last.bytes = len(data)
	return m.resp, m.err
}

// pdfBytes returns the embedded test PDF (see testdata/NOTICE.txt for
// provenance). One page, ~220 chars of text starting with
// "This is a heading".
func pdfBytes(t *testing.T) []byte {
	t.Helper()
	return helloPDF
}

func oversized(t *testing.T) []byte {
	t.Helper()
	return make([]byte, MaxBytes+1)
}

// --- table ---

func TestAnalyzer_Analyze(t *testing.T) {
	cases := []struct {
		name        string
		att         Attachment
		vc          VisionClient
		wantSkipped string
		wantErrIs   error
		wantTextHas string
	}{
		{
			name:        "PDF success: text is extracted",
			att:         Attachment{Filename: "kp.pdf", ContentType: "application/pdf", Data: pdfBytes(t)},
			vc:          &mockVisionClient{},
			wantTextHas: "This is a heading",
		},
		{
			name:        "PDF too large: skipped with too_large",
			att:         Attachment{Filename: "huge.pdf", ContentType: "application/pdf", Data: oversized(t)},
			vc:          &mockVisionClient{},
			wantSkipped: SkipTooLarge,
			wantErrIs:   ErrTooLarge,
		},
		{
			name:        "image too large: skipped with too_large",
			att:         Attachment{Filename: "big.png", ContentType: "image/png", Data: oversized(t)},
			vc:          &mockVisionClient{resp: "should not be called"},
			wantSkipped: SkipTooLarge,
			wantErrIs:   ErrTooLarge,
		},
		{
			name:        "image success: vision client returns text",
			att:         Attachment{Filename: "shot.png", ContentType: "image/png", Data: []byte("png-bytes")},
			vc:          &mockVisionClient{resp: "Backlog item: fix login bug"},
			wantTextHas: "Backlog item",
		},
		{
			name:        "image vision error: skipped with vision_error",
			att:         Attachment{Filename: "shot.png", ContentType: "image/png", Data: []byte("png-bytes")},
			vc:          &mockVisionClient{err: errors.New("rate limit")},
			wantSkipped: SkipVisionError,
		},
		{
			name:        "image without vision client: skipped with unsupported",
			att:         Attachment{Filename: "shot.png", ContentType: "image/png", Data: []byte("png-bytes")},
			vc:          nil,
			wantSkipped: SkipUnsupported,
			wantErrIs:   ErrUnsupportedFormat,
		},
		{
			name:        "unknown format: skipped with unsupported",
			att:         Attachment{Filename: "spec.docx", ContentType: "application/vnd.openxmlformats", Data: []byte("docx-bytes")},
			vc:          &mockVisionClient{},
			wantSkipped: SkipUnsupported,
			wantErrIs:   ErrUnsupportedFormat,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := New(tc.vc)
			res := a.Analyze(context.Background(), tc.att)

			if tc.wantSkipped != "" {
				assert.Equal(t, tc.wantSkipped, res.Skipped, "Skipped reason")
				if tc.wantErrIs != nil {
					assert.True(t, errors.Is(res.Err, tc.wantErrIs),
						"want errors.Is(..., %v), got %v", tc.wantErrIs, res.Err)
				}
				return
			}
			assert.Empty(t, res.Skipped, "expected success but got skip")
			assert.Contains(t, res.Text, tc.wantTextHas, "extracted text")
			assert.LessOrEqual(t, len(res.Preview), PreviewMax, "preview truncated")
		})
	}
}

func TestPreview_TruncatesLongText(t *testing.T) {
	long := strings.Repeat("ё", PreviewMax+50)
	got := preview(long)
	assert.LessOrEqual(t, len([]rune(got)), PreviewMax)
}

func TestPreview_PassesShortText(t *testing.T) {
	short := "short text"
	assert.Equal(t, short, preview(short))
}
