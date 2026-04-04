package outbound

import (
	"context"
	"fmt"
	"log"
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
	seqRepo      *sequences.Repository
	prospectRepo *prospects.Repository
}

// NewSender creates a sender that reads the Resend API key from user_settings (DB),
// falling back to the provided fallbackKey (from .env) if the DB value is empty.
func NewSender(store *settings.Store, ownerID uuid.UUID, fallbackKey, fromAddress, appBaseURL string, seqRepo *sequences.Repository, prospectRepo *prospects.Repository) *Sender {
	return &Sender{
		store:        store,
		ownerID:      ownerID,
		fallbackKey:  fallbackKey,
		fromAddress:  fromAddress,
		appBaseURL:   appBaseURL,
		seqRepo:      seqRepo,
		prospectRepo: prospectRepo,
	}
}

// SendPending finds all approved email messages ready to send and sends them via Resend.
func (s *Sender) SendPending(ctx context.Context) error {
	// Resolve Resend API key: DB first, then .env fallback
	apiKey := s.fallbackKey
	if cfg, err := s.store.GetConfig(ctx, s.ownerID); err == nil && cfg.ResendAPIKey != "" {
		apiKey = cfg.ResendAPIKey
	}
	if apiKey == "" {
		return nil // no API key configured -- skip silently
	}

	msgs, err := s.seqRepo.GetPendingSends(ctx)
	if err != nil {
		return fmt.Errorf("get pending sends: %w", err)
	}

	client := resend.NewClient(apiKey)

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

		trackingPixel := ""
		if s.appBaseURL != "" {
			trackingPixel = fmt.Sprintf(`<img src="%s/api/track/open/%s" width="1" height="1" style="display:none" />`, s.appBaseURL, msg.ID)
		}

		params := &resend.SendEmailRequest{
			From:    s.fromAddress,
			To:      []string{prospect.Email},
			Subject: "Сообщение от Floq",
			Html:    "<html><body>" + msg.Body + trackingPixel + "</body></html>",
		}

		_, err = client.Emails.Send(params)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "bounce") || strings.Contains(errStr, "invalid email") || strings.Contains(errStr, "recipient rejected") || strings.Contains(errStr, "mailbox not found") {
				log.Printf("[outbound] bounce detected for %s (msg %s): %v", prospect.Email, msg.ID, err)
				if markErr := s.seqRepo.MarkBounced(ctx, msg.ID); markErr != nil {
					log.Printf("[outbound] failed to mark message %s as bounced: %v", msg.ID, markErr)
				}
				if markErr := s.prospectRepo.UpdateVerification(ctx, msg.ProspectID, prospectsdomain.VerifyStatusInvalid, 0, `{"bounce":true}`, time.Now().UTC()); markErr != nil {
					log.Printf("[outbound] failed to mark prospect %s as invalid: %v", msg.ProspectID, markErr)
				}
				continue
			}
			log.Printf("[outbound] failed to send email to %s (msg %s): %v", prospect.Email, msg.ID, err)
			continue
		}

		if err := s.seqRepo.MarkSent(ctx, msg.ID); err != nil {
			log.Printf("[outbound] failed to mark message %s as sent: %v", msg.ID, err)
			continue
		}

		log.Printf("[outbound] sent email to %s (msg %s)", prospect.Email, msg.ID)
	}

	return nil
}
