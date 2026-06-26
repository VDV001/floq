// Package normalize provides pure functions for canonicalizing contact
// identifiers (email, phone, telegram username) used by domain factories.
//
// The package lives outside the domain layer because the same rules are
// shared across bounded contexts (leads, prospects, inbox) — duplicating the
// logic per context would let identifiers diverge subtly (e.g. one context
// lowercases email, another does not, breaking cross-channel matching).
//
// All functions are pure: same input → same output, no I/O, no clock.
package normalize

import "strings"

// Email returns the canonical form of an email address: trimmed of leading
// and trailing whitespace and lowercased. Empty input maps to empty output.
//
// Lowercasing is safe because the local-part of SMTP addresses is
// case-insensitive in every mail server we interact with (Resend, Gmail,
// Yandex, corp providers). Treating "ALICE@acme.com" and "alice@acme.com"
// as distinct identifiers would silently fork the same prospect.
func Email(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// Phone returns the canonical form of a phone number: a leading "+" (if the
// input started with one) followed by digits only. Spaces, dashes, dots,
// parentheses, and other separators are stripped. Returns empty string if
// the input contains no digits.
//
// We do not convert "8800…" to "+7800…" — that rule is country-specific and
// would corrupt non-RU numbers. Cross-context matching uses the canonical
// digit string; the "+" is preserved for E.164 round-trips when present.
func Phone(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	hasPlus := strings.HasPrefix(s, "+")
	var b strings.Builder
	b.Grow(len(s) + 1)
	if hasPlus {
		b.WriteByte('+')
	}
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" || out == "+" {
		return ""
	}
	return out
}

// TelegramUsername returns the canonical form of a Telegram handle: trimmed
// of whitespace, stripped of leading "@", and lowercased. Telegram itself
// treats usernames case-insensitively, so matching by lowercased form is
// the only way to dedupe correctly.
func TelegramUsername(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "@")
	return strings.ToLower(strings.TrimSpace(s))
}
