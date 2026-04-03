package outbound

import (
	"context"
	"fmt"
	"log"

	resend "github.com/resendlabs/resend-go"

	"github.com/daniil/floq/internal/prospects"
	"github.com/daniil/floq/internal/sequences"
	"github.com/daniil/floq/internal/settings"
	"github.com/google/uuid"
)

type Sender struct {
	store        *settings.Store
	ownerID      uuid.UUID
	fallbackKey  string
	fromAddress  string
	seqRepo      *sequences.Repository
	prospectRepo *prospects.Repository
}

// NewSender creates a sender that reads the Resend API key from user_settings (DB),
// falling back to the provided fallbackKey (from .env) if the DB value is empty.
func NewSender(store *settings.Store, ownerID uuid.UUID, fallbackKey, fromAddress string, seqRepo *sequences.Repository, prospectRepo *prospects.Repository) *Sender {
	return &Sender{
		store:        store,
		ownerID:      ownerID,
		fallbackKey:  fallbackKey,
		fromAddress:  fromAddress,
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
		return nil // no API key configured — skip silently
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

		params := &resend.SendEmailRequest{
			From:    s.fromAddress,
			To:      []string{prospect.Email},
			Subject: "Сообщение от Floq",
			Html:    "<html><body>" + msg.Body + "</body></html>",
		}

		_, err = client.Emails.Send(params)
		if err != nil {
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
