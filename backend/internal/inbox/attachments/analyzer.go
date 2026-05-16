// Package attachments analyses email/Telegram attachments (PDFs,
// screenshots) and returns the text content for downstream AI
// qualification. The package owns three concerns:
//
//   - PDF text extraction via ledongthuc/pdf (no CGO, text-layer only).
//   - Image OCR via an injected VisionClient (gpt-4o-mini in production).
//   - Routing by MIME type plus size and page-count limits so a single
//     bad attachment cannot stall the inbound pipeline.
//
// Failure modes are surfaced through Result.Skipped + Result.Err rather
// than panics or hard errors so callers can degrade gracefully: a lead
// is created without attachment context, never lost because the
// analyser is down.
package attachments

import (
	"context"
	"errors"
	"strings"
)

// Limits applied uniformly to PDF and image attachments. Tuned for B2B
// inbound (КП on a few pages, screenshot of a backlog/error) — anything
// bigger almost certainly needs a human to look at it anyway.
const (
	MaxPDFPages = 10
	MaxBytes    = 5 * 1024 * 1024 // 5 MB
	PreviewMax  = 1024            // chars of extracted text logged on success
)

// Sentinel errors. Callers should match via errors.Is to decide whether
// to log, retry, or surface the skip to the operator.
var (
	ErrTooLarge          = errors.New("attachment exceeds size limit")
	ErrTooManyPages      = errors.New("PDF exceeds page limit")
	ErrUnsupportedFormat = errors.New("unsupported attachment format")
	ErrNoTextLayer       = errors.New("PDF has no extractable text layer")
)

// Skip reasons surfaced in Result.Skipped. Stable strings so they can
// be matched in logs / metrics.
const (
	SkipTooLarge      = "too_large"
	SkipTooManyPages  = "too_many_pages"
	SkipUnsupported   = "unsupported"
	SkipNoTextLayer   = "no_text"
	SkipVisionError   = "vision_error"
	SkipExtractError  = "extract_error"
)

// Attachment is the raw payload the analyser receives.
type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// Result is the analyser's verdict for one attachment. Either Text is
// populated (success) or Skipped + Err are (graceful skip). Pages is
// non-zero only for PDF success. Preview is Text truncated to
// PreviewMax bytes (rune-safe) so logs don't blow up on a 10-page КП.
type Result struct {
	Filename    string
	ContentType string
	Pages       int
	Text        string
	Preview     string
	Skipped     string
	Err         error
}

// VisionClient is the minimum interface the analyser needs for image
// OCR. AIClient implements it when its provider supports vision; the
// production wiring lives in cmd/server.
type VisionClient interface {
	AnalyzeImage(ctx context.Context, imageData []byte, mimeType, prompt string) (string, error)
}

// Analyzer routes Attachments by MIME type. Construct via New so future
// options (custom limits, custom prompt for vision) plug in without
// breaking call sites.
type Analyzer struct {
	vc VisionClient
}

// New returns an Analyzer that hands image attachments to vc. PDF
// analysis does not require vc; pass nil if vision is disabled and
// expect image attachments to be skipped with SkipUnsupported.
func New(vc VisionClient) *Analyzer {
	return &Analyzer{vc: vc}
}

// Analyze inspects one attachment and returns its extracted text plus
// metadata. It does not return an error directly — failures are
// reported through Result.Skipped + Result.Err so a batch of
// attachments can be processed with a uniform loop.
//
// Routing: any payload over MaxBytes is skipped before MIME inspection
// so a 50 MB PDF never even reaches the parser. PDFs go through the
// text-layer extractor; image/* MIME types go through the VisionClient;
// everything else is skipped as SkipUnsupported.
func (a *Analyzer) Analyze(ctx context.Context, att Attachment) Result {
	res := Result{Filename: att.Filename, ContentType: att.ContentType}

	if len(att.Data) > MaxBytes {
		res.Skipped = SkipTooLarge
		res.Err = ErrTooLarge
		return res
	}

	mime := strings.ToLower(strings.TrimSpace(att.ContentType))
	switch {
	case strings.HasPrefix(mime, "application/pdf"):
		text, pages, err := extractPDFText(att.Data)
		res.Pages = pages
		switch {
		case errors.Is(err, ErrTooManyPages):
			res.Skipped = SkipTooManyPages
			res.Err = err
			return res
		case errors.Is(err, ErrNoTextLayer):
			res.Skipped = SkipNoTextLayer
			res.Err = err
			return res
		case err != nil:
			res.Skipped = SkipExtractError
			res.Err = err
			return res
		}
		res.Text = text
		res.Preview = preview(text)
		return res

	case strings.HasPrefix(mime, "image/"):
		if a.vc == nil {
			res.Skipped = SkipUnsupported
			res.Err = ErrUnsupportedFormat
			return res
		}
		text, err := extractImageText(ctx, a.vc, att.Data, mime)
		if err != nil {
			res.Skipped = SkipVisionError
			res.Err = err
			return res
		}
		res.Text = text
		res.Preview = preview(text)
		return res

	default:
		res.Skipped = SkipUnsupported
		res.Err = ErrUnsupportedFormat
		return res
	}
}

// preview returns the first PreviewMax runes of text. Exposed for tests;
// callers should read Result.Preview instead of computing it twice.
func preview(text string) string {
	text = strings.TrimSpace(text)
	if len(text) <= PreviewMax {
		return text
	}
	runes := []rune(text)
	if len(runes) <= PreviewMax {
		return text
	}
	return string(runes[:PreviewMax])
}
