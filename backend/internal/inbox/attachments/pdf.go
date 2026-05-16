package attachments

// extractPDFText is the RED stub. The GREEN commit reads the PDF with
// github.com/ledongthuc/pdf, enforces MaxPDFPages, returns the text
// content joined across pages (or ErrNoTextLayer when the PDF has no
// extractable text).
func extractPDFText(_ []byte) (text string, pages int, err error) {
	return "", 0, ErrUnsupportedFormat
}
