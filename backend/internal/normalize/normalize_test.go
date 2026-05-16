package normalize_test

import (
	"testing"

	"github.com/daniil/floq/internal/normalize"
)

func TestEmail(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"all caps", "ALICE@ACME.COM", "alice@acme.com"},
		{"mixed case", "Alice@Acme.Com", "alice@acme.com"},
		{"leading and trailing whitespace", "  alice@acme.com  ", "alice@acme.com"},
		{"tab and newline", "\talice@acme.com\n", "alice@acme.com"},
		{"already canonical", "alice@acme.com", "alice@acme.com"},
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := normalize.Email(c.in); got != c.want {
				t.Errorf("Email(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestPhone(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"e164 ru with spaces and parens", "+7 999 (123) 45-67", "+79991234567"},
		{"e164 ru with dashes", "+7-999-123-45-67", "+79991234567"},
		{"local ru with parens", "8 (800) 000-00-00", "88000000000"},
		{"e164 us with dots", "+1.408.555.0123", "+14085550123"},
		{"e164 uk with surrounding whitespace", "  +44 20 7946 0958  ", "+442079460958"},
		{"already canonical", "+79991234567", "+79991234567"},
		{"plus only", "+", ""},
		{"no digits", "not a phone", ""},
		{"empty", "", ""},
		{"only separators", "()- ", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := normalize.Phone(c.in); got != c.want {
				t.Errorf("Phone(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestTelegramUsername(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"leading at", "@alice_bot", "alice_bot"},
		{"leading at mixed case", "@Alice_Bot", "alice_bot"},
		{"no at sign", "Alice", "alice"},
		{"surrounding whitespace", "  @alice  ", "alice"},
		{"whitespace and at", "  @  ", ""},
		{"empty", "", ""},
		{"already canonical", "alice_bot", "alice_bot"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := normalize.TelegramUsername(c.in); got != c.want {
				t.Errorf("TelegramUsername(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
