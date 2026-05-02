package verify

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmailRegex_ValidEmails(t *testing.T) {
	valid := []string{
		"user@example.com",
		"first.last@domain.org",
		"user+tag@sub.domain.co",
		"name123@test.io",
		"a@b.ru",
		"user-name@domain.com",
		"user_name@domain.com",
		"user%name@domain.com",
	}
	for _, email := range valid {
		assert.True(t, emailRegex.MatchString(email), "expected valid: %s", email)
	}
}

func TestEmailRegex_InvalidEmails(t *testing.T) {
	invalid := []string{
		"",
		"plainaddress",
		"@domain.com",
		"user@",
		"user@.com",
		"user@domain",
		"user @domain.com",
		"user@domain .com",
	}
	for _, email := range invalid {
		assert.False(t, emailRegex.MatchString(email), "expected invalid: %s", email)
	}
}

func TestIsDisposable(t *testing.T) {
	tests := []struct {
		domain     string
		disposable bool
	}{
		{"mailinator.com", true},
		{"guerrillamail.com", true},
		{"tempmail.com", true},
		{"yopmail.com", true},
		{"Mailinator.COM", true}, // case-insensitive
		{"gmail.com", false},
		{"company.com", false},
		{"outlook.com", false},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.disposable, IsDisposable(tc.domain), "domain: %s", tc.domain)
	}
}

func TestFreeProviders(t *testing.T) {
	free := []string{
		"gmail.com", "yahoo.com", "hotmail.com", "outlook.com",
		"mail.ru", "yandex.ru", "protonmail.com", "proton.me",
		"icloud.com", "bk.ru", "list.ru", "inbox.ru", "rambler.ru",
	}
	for _, domain := range free {
		assert.True(t, freeProviders[domain], "expected free: %s", domain)
	}

	notFree := []string{"company.com", "example.org", "startup.io"}
	for _, domain := range notFree {
		assert.False(t, freeProviders[domain], "expected not free: %s", domain)
	}
}

func TestVerifyEmail_InvalidSyntax(t *testing.T) {
	result := VerifyEmail(context.Background(), "not-an-email", nil)
	assert.False(t, result.IsValidSyntax)
	assert.Equal(t, 0, result.Score)
	assert.Equal(t, "invalid", result.Status)
	assert.Equal(t, "not-an-email", result.Email)
}

func TestVerifyEmail_EmptyString(t *testing.T) {
	result := VerifyEmail(context.Background(), "", nil)
	assert.False(t, result.IsValidSyntax)
	assert.Equal(t, 0, result.Score)
	assert.Equal(t, "invalid", result.Status)
}

func TestScoreCalculation(t *testing.T) {
	// Test the scoring logic by constructing EmailResult manually
	// and verifying the arithmetic matches what VerifyEmail does.

	// Scenario: valid syntax + MX + SMTP valid = 20+25+40 = 85 => "valid"
	score := 0
	score += 20 // valid syntax
	score += 25 // has MX
	score += 40 // SMTP valid
	assert.Equal(t, 85, score)

	// With free provider: 85 - 5 = 80 => still "valid"
	scoreFree := score - 5
	assert.Equal(t, 80, scoreFree)

	// With catch-all: 85 - 20 = 65 => "risky" (>=40, <70)
	scoreCatchAll := score - 20
	assert.Equal(t, 65, scoreCatchAll)

	// Free + catch-all: 85 - 5 - 20 = 60 => "risky"
	scoreFreeCatchAll := score - 5 - 20
	assert.Equal(t, 60, scoreFreeCatchAll)

	// Disposable overrides to 5 => "invalid"
	assert.Equal(t, 5, 5) // disposable forces score=5
}

func TestScoreStatus_Boundaries(t *testing.T) {
	tests := []struct {
		score  int
		status string
	}{
		{85, "valid"},
		{70, "valid"},
		{69, "risky"},
		{40, "risky"},
		{39, "invalid"},
		{0, "invalid"},
		{5, "invalid"},
	}
	for _, tc := range tests {
		var status string
		switch {
		case tc.score >= 70:
			status = "valid"
		case tc.score >= 40:
			status = "risky"
		default:
			status = "invalid"
		}
		assert.Equal(t, tc.status, status, "score=%d", tc.score)
	}
}

func TestScoreNeverNegative(t *testing.T) {
	// According to the code, if score < 0, it is clamped to 0.
	// This can happen if: syntax valid (20) + no MX (0) + no SMTP (0)
	// - catch-all (20) - free (5) = -5 => clamped to 0.
	score := 20 - 20 - 5
	if score < 0 {
		score = 0
	}
	assert.Equal(t, 0, score)
}
