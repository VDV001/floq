package verify

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"regexp"
	"strings"
	"time"

	"github.com/daniil/floq/internal/prospects/domain"
	"github.com/daniil/floq/internal/proxy"
)

// EmailResult holds the outcome of an email verification pipeline.
type EmailResult struct {
	Email          string              `json:"email"`
	IsValidSyntax  bool                `json:"is_valid_syntax"`
	HasMX          bool                `json:"has_mx"`
	SMTPValid      bool                `json:"smtp_valid"`
	SMTPError      string              `json:"smtp_error,omitempty"`
	IsDisposable   bool                `json:"is_disposable"`
	IsCatchAll     bool                `json:"is_catch_all"`
	IsFreeProvider bool                `json:"is_free_provider"`
	Score          int                 `json:"score"`
	Status         domain.VerifyStatus `json:"status"`
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

var freeProviders = map[string]bool{
	"gmail.com":      true,
	"yahoo.com":      true,
	"hotmail.com":    true,
	"outlook.com":    true,
	"mail.ru":        true,
	"yandex.ru":      true,
	"yandex.com":     true,
	"bk.ru":          true,
	"list.ru":        true,
	"inbox.ru":       true,
	"rambler.ru":     true,
	"icloud.com":     true,
	"protonmail.com": true,
	"proton.me":      true,
}

// VerifyEmail runs the full email verification pipeline and returns the result.
// dialer may be nil for a direct connection.
func VerifyEmail(ctx context.Context, email string, dialer proxy.ContextDialer) EmailResult {
	result := EmailResult{
		Email: email,
	}

	// Step 1: Syntax check
	if !emailRegex.MatchString(email) {
		result.Score = 0
		result.Status = domain.VerifyStatusInvalid
		return result
	}
	result.IsValidSyntax = true

	// Step 2: Extract domain
	parts := strings.SplitN(email, "@", 2)
	emailDomain := parts[1]

	// Step 3: Disposable check
	result.IsDisposable = IsDisposable(emailDomain)

	// Step 4: Free provider check
	result.IsFreeProvider = freeProviders[strings.ToLower(emailDomain)]

	// Step 5: MX lookup
	mxRecords, err := net.LookupMX(emailDomain)
	if err != nil || len(mxRecords) == 0 {
		result.Score = 5
		result.Status = domain.VerifyStatusInvalid
		return result
	}
	result.HasMX = true

	// Step 6: SMTP probe
	mxHost := strings.TrimSuffix(mxRecords[0].Host, ".")
	smtpValid, catchAll, smtpErr := smtpProbe(ctx, mxHost, email, emailDomain, dialer)
	result.SMTPValid = smtpValid
	result.IsCatchAll = catchAll
	if smtpErr != "" {
		result.SMTPError = smtpErr
	}

	// Step 8: Scoring
	score := 0
	if result.IsValidSyntax {
		score += 20
	}
	if result.HasMX {
		score += 25
	}
	if result.SMTPValid {
		score += 40
	}
	if result.IsCatchAll {
		score -= 20
	}
	if result.IsFreeProvider {
		score -= 5
	}
	if result.IsDisposable {
		score = 5
	}
	if score < 0 {
		score = 0
	}
	result.Score = score

	// Step 9: Status
	switch {
	case score >= 70:
		result.Status = domain.VerifyStatusValid
	case score >= 40:
		result.Status = domain.VerifyStatusRisky
	default:
		result.Status = domain.VerifyStatusInvalid
	}

	return result
}

// smtpProbe connects to the MX host and checks whether the email is deliverable.
// It also performs catch-all detection. Returns (smtpValid, isCatchAll, errorMessage).
func smtpProbe(ctx context.Context, mxHost, email, emailDomain string, dialer proxy.ContextDialer) (bool, bool, string) {
	addr := mxHost + ":25"
	var conn net.Conn
	var err error
	if dialer != nil {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	} else {
		netDialer := &net.Dialer{Timeout: 10 * time.Second}
		conn, err = netDialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return false, false, fmt.Sprintf("dial error: %v", err)
	}

	client, err := smtp.NewClient(conn, mxHost)
	if err != nil {
		conn.Close()
		return false, false, fmt.Sprintf("smtp client error: %v", err)
	}
	defer client.Close()
	defer client.Quit()

	// EHLO
	if err := client.Hello("verify.floq.app"); err != nil {
		return false, false, fmt.Sprintf("EHLO error: %v", err)
	}

	// MAIL FROM
	if err := client.Mail("check@floq.app"); err != nil {
		return false, false, fmt.Sprintf("MAIL FROM error: %v", err)
	}

	// Step 6: RCPT TO — the real email
	smtpValid := true
	if err := client.Rcpt(email); err != nil {
		return false, false, fmt.Sprintf("RCPT TO error: %v", err)
	}

	// Step 7: Catch-all detection — try a random address
	catchAll := false
	randomAddr := fmt.Sprintf("floq-verify-test-%d@%s", time.Now().UnixNano(), emailDomain)

	// Reset for the catch-all probe
	if err := client.Reset(); err == nil {
		if err := client.Mail("check@floq.app"); err == nil {
			if err := client.Rcpt(randomAddr); err == nil {
				catchAll = true
			}
		}
	}

	return smtpValid, catchAll, ""
}
