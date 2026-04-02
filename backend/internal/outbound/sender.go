package outbound

import (
	"context"
	"fmt"
	"log"

	resend "github.com/resendlabs/resend-go"

	"github.com/daniil/floq/internal/prospects"
	"github.com/daniil/floq/internal/sequences"
)

type Sender struct {
	resendClient *resend.Client
	fromAddress  string
	seqRepo      *sequences.Repository
	prospectRepo *prospects.Repository
}

func NewSender(apiKey, fromAddress string, seqRepo *sequences.Repository, prospectRepo *prospects.Repository) *Sender {
	return &Sender{
		resendClient: resend.NewClient(apiKey),
		fromAddress:  fromAddress,
		seqRepo:      seqRepo,
		prospectRepo: prospectRepo,
	}
}

// SendPending finds all approved email messages ready to send and sends them via Resend.
func (s *Sender) SendPending(ctx context.Context) error {
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

		params := &resend.SendEmailRequest{
			From:    s.fromAddress,
			To:      []string{prospect.Email},
			Subject: "Сообщение от Floq",
			Html:    "<html><body>" + msg.Body + "</body></html>",
		}

		_, err = s.resendClient.Emails.Send(params)
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
