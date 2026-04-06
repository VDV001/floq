package outbound

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"strings"
	"time"

	resend "github.com/resendlabs/resend-go"

	"github.com/daniil/floq/internal/prospects"
	prospectsdomain "github.com/daniil/floq/internal/prospects/domain"
	"github.com/daniil/floq/internal/sequences"
	"github.com/daniil/floq/internal/settings"
	"github.com/google/uuid"
)

type Sender struct {
	store        *settings.Store
	ownerID      uuid.UUID
	fallbackKey  string
	fromAddress  string
	appBaseURL   string
	smtpHost     string
	smtpPort     string
	smtpUser     string
	smtpPassword string
	seqRepo      *sequences.Repository
	prospectRepo *prospects.Repository
}

func NewSender(
	store *settings.Store, ownerID uuid.UUID,
	fallbackKey, fromAddress, appBaseURL string,
	smtpHost, smtpPort, smtpUser, smtpPassword string,
	seqRepo *sequences.Repository, prospectRepo *prospects.Repository,
) *Sender {
	return &Sender{
		store:        store,
		ownerID:      ownerID,
		fallbackKey:  fallbackKey,
		fromAddress:  fromAddress,
		appBaseURL:   appBaseURL,
		smtpHost:     smtpHost,
		smtpPort:     smtpPort,
		smtpUser:     smtpUser,
		smtpPassword: smtpPassword,
		seqRepo:      seqRepo,
		prospectRepo: prospectRepo,
	}
}

// SendPending finds all approved email messages ready to send.
// Uses SMTP if configured (DB first, then .env), otherwise falls back to Resend API.
func (s *Sender) SendPending(ctx context.Context) error {
	// Resolve SMTP/Resend settings from DB, fallback to .env
	smtpHost, smtpPort, smtpUser, smtpPassword := s.smtpHost, s.smtpPort, s.smtpUser, s.smtpPassword
	fromAddr := s.fromAddress
	if cfg, err := s.store.GetConfig(ctx, s.ownerID); err == nil {
		if cfg.SMTPHost != "" {
			smtpHost = cfg.SMTPHost
		}
		if cfg.SMTPPort != "" {
			smtpPort = cfg.SMTPPort
		}
		if cfg.SMTPUser != "" {
			smtpUser = cfg.SMTPUser
		}
		if cfg.SMTPPassword != "" {
			smtpPassword = cfg.SMTPPassword
		}
		// Use SMTP user as from address if no explicit from set
		if smtpUser != "" && fromAddr == "" {
			fromAddr = smtpUser
		}
	}

	msgs, err := s.seqRepo.GetPendingSends(ctx)
	if err != nil {
		return fmt.Errorf("get pending sends: %w", err)
	}

	for _, msg := range msgs {
		if msg.Channel != "email" {
			continue
		}

		prospect, err := s.prospectRepo.GetProspect(ctx, msg.ProspectID)
		if err != nil {
			log.Printf("[outbound] error fetching prospect %s: %v", msg.ProspectID, err)
			continue
		}
		if prospect == nil || prospect.Email == "" {
			continue
		}

		subject := fmt.Sprintf("%s, сотрудничество с %s", prospect.Name, prospect.Company)
		if prospect.Company == "" {
			subject = fmt.Sprintf("%s, предложение о сотрудничестве", prospect.Name)
		}

		trackingPixel := ""
		if s.appBaseURL != "" {
			trackingPixel = fmt.Sprintf(`<img src="%s/api/track/open/%s" width="1" height="1" style="display:none" />`, s.appBaseURL, msg.ID)
		}

		htmlBody := "<html><body>" + msg.Body + trackingPixel + "</body></html>"

		var sendErr error
		if smtpHost != "" && smtpUser != "" && smtpPassword != "" {
			sendErr = s.sendViaSMTPWith(smtpHost, smtpPort, smtpUser, smtpPassword, fromAddr, prospect.Email, subject, htmlBody)
		} else {
			sendErr = s.sendViaResend(ctx, prospect.Email, subject, htmlBody)
		}

		if sendErr != nil {
			errStr := sendErr.Error()
			if strings.Contains(errStr, "bounce") || strings.Contains(errStr, "invalid") || strings.Contains(errStr, "rejected") || strings.Contains(errStr, "mailbox") || strings.Contains(errStr, "550") || strings.Contains(errStr, "553") {
				log.Printf("[outbound] bounce for %s (msg %s): %v", prospect.Email, msg.ID, sendErr)
				_ = s.seqRepo.MarkBounced(ctx, msg.ID)
				_ = s.prospectRepo.UpdateVerification(ctx, msg.ProspectID, prospectsdomain.VerifyStatusInvalid, 0, `{"bounce":true}`, time.Now().UTC())
				continue
			}
			log.Printf("[outbound] send failed to %s (msg %s): %v", prospect.Email, msg.ID, sendErr)
			continue
		}

		if err := s.seqRepo.MarkSent(ctx, msg.ID); err != nil {
			log.Printf("[outbound] failed to mark %s as sent: %v", msg.ID, err)
			continue
		}

		log.Printf("[outbound] sent email to %s (msg %s)", prospect.Email, msg.ID)
	}

	return nil
}

// sendViaSMTPWith sends email through an SMTP server (mail.ru, Yandex, Gmail, etc.)
func (s *Sender) sendViaSMTPWith(host, port, user, password, from, to, subject, htmlBody string) error {
	if from == "" {
		from = user
	}

	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=\"utf-8\"\r\n\r\n",
		from, to, subject)
	message := []byte(headers + htmlBody)

	addr := host + ":" + port
	auth := smtp.PlainAuth("", user, password, host)

	// mail.ru / Yandex require TLS on port 465
	if port == "465" {
		return s.sendSMTPWithTLS(addr, auth, host, from, to, message)
	}

	// Port 587 uses STARTTLS
	return smtp.SendMail(addr, auth, from, []string{to}, message)
}

// sendSMTPWithTLS handles implicit TLS (port 465).
func (s *Sender) sendSMTPWithTLS(addr string, auth smtp.Auth, host, from, to string, message []byte) error {
	tlsConfig := &tls.Config{ServerName: host}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("smtp tls dial: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(message); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}

	return client.Quit()
}

// sendViaResend sends email through the Resend API.
func (s *Sender) sendViaResend(ctx context.Context, to, subject, htmlBody string) error {
	apiKey := s.fallbackKey
	if cfg, err := s.store.GetConfig(ctx, s.ownerID); err == nil && cfg.ResendAPIKey != "" {
		apiKey = cfg.ResendAPIKey
	}
	if apiKey == "" {
		return fmt.Errorf("no Resend API key configured")
	}

	client := resend.NewClient(apiKey)
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    s.fromAddress,
		To:      []string{to},
		Subject: subject,
		Html:    htmlBody,
	})
	return err
}
