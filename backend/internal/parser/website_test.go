package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractEmails_PlainText(t *testing.T) {
	html := `Contact us at info@example.com or sales@company.org for details.`
	emails := extractEmails(html)
	assert.Contains(t, emails, "info@example.com")
	assert.Contains(t, emails, "sales@company.org")
}

func TestExtractEmails_MailtoLinks(t *testing.T) {
	html := `<a href="mailto:Support@Example.COM">Email us</a>`
	emails := extractEmails(html)
	assert.Contains(t, emails, "support@example.com")
}

func TestExtractEmails_Deduplication(t *testing.T) {
	html := `
		<a href="mailto:hello@test.com">link</a>
		Contact: hello@test.com
		Also: HELLO@TEST.COM
	`
	emails := extractEmails(html)
	count := 0
	for _, e := range emails {
		if e == "hello@test.com" {
			count++
		}
	}
	assert.Equal(t, 1, count, "expected exactly one occurrence of hello@test.com")
}

func TestExtractEmails_Empty(t *testing.T) {
	emails := extractEmails("<html><body>No emails here</body></html>")
	assert.Empty(t, emails)
}

func TestExtractEmails_MailtoWithQuery(t *testing.T) {
	html := `<a href="mailto:info@example.com?subject=Hello">email</a>`
	emails := extractEmails(html)
	assert.Contains(t, emails, "info@example.com")
	// The query part should not be included
	for _, e := range emails {
		assert.NotContains(t, e, "?")
	}
}

func TestIsJunkEmail(t *testing.T) {
	junks := []string{
		"noreply@example.com",
		"no-reply@test.org",
		"no_reply@company.com",
		"mailer-daemon@server.com",
		"postmaster@mail.com",
		"webmaster@site.org",
		"test@anything.com",
		"admin@foo.bar",
	}
	for _, email := range junks {
		assert.True(t, isJunkEmail(email), "expected junk: %s", email)
	}
}

func TestIsJunkEmail_NotJunk(t *testing.T) {
	notJunks := []string{
		"info@example.com",
		"sales@company.org",
		"ceo@startup.io",
		"hello@world.com",
		"testing-dept@company.com", // "testing" starts with "test" but no "@" in prefix rule; check local part
	}
	for _, email := range notJunks {
		assert.False(t, isJunkEmail(email), "expected not junk: %s", email)
	}
}

func TestIsJunkEmail_CaseInsensitive(t *testing.T) {
	assert.True(t, isJunkEmail("NoReply@Example.COM"))
	assert.True(t, isJunkEmail("ADMIN@domain.com"))
}

func TestContactPaths(t *testing.T) {
	expected := []string{
		"/contacts", "/contact", "/kontakty",
		"/about", "/about-us", "/o-kompanii",
	}
	assert.Equal(t, expected, contactPaths)
}
