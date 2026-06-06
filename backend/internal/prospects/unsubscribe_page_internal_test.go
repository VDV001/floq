package prospects

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWriteUnsubscribePage_EscapesMessage guards the public, unauthenticated
// unsubscribe page against HTML injection: the message must be escaped, so the
// function is safe even if a caller ever passes non-static text.
func TestWriteUnsubscribePage_EscapesMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	writeUnsubscribePage(rec, 200, "<script>alert(1)</script>")

	body := rec.Body.String()
	if strings.Contains(body, "<script>") {
		t.Errorf("message must be HTML-escaped, got raw <script> in:\n%s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Errorf("expected escaped entity in body:\n%s", body)
	}
}
