package inbox

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/leads"
)

// EmailPoller polls an IMAP mailbox for new emails and creates leads.
type EmailPoller struct {
	host     string
	port     string
	user     string
	password string
	pool     *pgxpool.Pool
	repo     *leads.Repository
	aiClient *ai.AIClient
	ownerID  uuid.UUID
}

// NewEmailPoller creates a new EmailPoller with the given IMAP credentials and dependencies.
func NewEmailPoller(host, port, user, password string, pool *pgxpool.Pool, repo *leads.Repository, aiClient *ai.AIClient, ownerID uuid.UUID) *EmailPoller {
	return &EmailPoller{
		host:     host,
		port:     port,
		user:     user,
		password: password,
		pool:     pool,
		repo:     repo,
		aiClient: aiClient,
		ownerID:  ownerID,
	}
}

// Start begins polling the IMAP mailbox for new emails.
// It blocks until ctx is cancelled.
func (e *EmailPoller) Start(ctx context.Context) {
	log.Println("Email polling started")
	log.Println("Email polling not fully implemented (IMAP stub)")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Email poller shutting down")
			return
		case <-ticker.C:
			e.poll(ctx)
		}
	}
}

// poll checks the mailbox for new emails. This is a stub for MVP.
func (e *EmailPoller) poll(ctx context.Context) {
	// TODO: Implement full IMAP connection using crypto/tls + net.
	// For each new email:
	//   1. Parse From header for contact name and email address
	//   2. Check if lead with this email_address already exists
	//   3. If not, create new Lead (channel="email")
	//   4. Create inbound Message
	//   5. Trigger async qualification for new leads
	//
	// Skeleton for processing a new email (not called yet):
	// e.processEmail(ctx, fromName, fromEmail, subject, body)
	_ = ctx
}

// processEmail handles a single inbound email, creating or updating leads.
// This will be called by poll() once IMAP fetching is implemented.
func (e *EmailPoller) processEmail(ctx context.Context, fromName, fromEmail, body string) {
	existing, err := e.repo.GetLeadByEmailAddress(ctx, e.ownerID, fromEmail)
	if err != nil {
		log.Printf("email inbox: error looking up lead for %s: %v", fromEmail, err)
		return
	}

	isNewLead := existing == nil
	var lead *leads.Lead

	if isNewLead {
		emailAddr := fromEmail
		lead = &leads.Lead{
			ID:           uuid.New(),
			UserID:       e.ownerID,
			Channel:      "email",
			ContactName:  fromName,
			Company:      "",
			FirstMessage: body,
			Status:       "new",
			EmailAddress: &emailAddr,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		if err := e.repo.CreateLead(ctx, lead); err != nil {
			log.Printf("email inbox: error creating lead: %v", err)
			return
		}
		log.Printf("email inbox: new lead created for %s (%s)", fromEmail, fromName)
	} else {
		lead = existing
	}

	message := &leads.Message{
		ID:        uuid.New(),
		LeadID:    lead.ID,
		Direction: "inbound",
		Body:      body,
		SentAt:    time.Now().UTC(),
	}
	if err := e.repo.CreateMessage(ctx, message); err != nil {
		log.Printf("email inbox: error creating message: %v", err)
		return
	}

	if isNewLead {
		go func() {
			qCtx := context.Background()
			result, err := e.aiClient.Qualify(qCtx, fromName, lead.Channel, lead.FirstMessage)
			if err != nil {
				log.Printf("email inbox: qualification error for lead %s: %v", lead.ID, err)
				return
			}

			q := &leads.Qualification{
				ID:                uuid.New(),
				LeadID:            lead.ID,
				IdentifiedNeed:    result.IdentifiedNeed,
				EstimatedBudget:   result.EstimatedBudget,
				Deadline:          result.Deadline,
				Score:             result.Score,
				ScoreReason:       result.ScoreReason,
				RecommendedAction: result.RecommendedAction,
				ProviderUsed:      e.aiClient.ProviderName(),
				GeneratedAt:       time.Now().UTC(),
			}
			if err := e.repo.UpsertQualification(qCtx, q); err != nil {
				log.Printf("email inbox: error saving qualification for lead %s: %v", lead.ID, err)
				return
			}
			if err := e.repo.UpdateLeadStatus(qCtx, lead.ID, "qualified"); err != nil {
				log.Printf("email inbox: error updating lead status for %s: %v", lead.ID, err)
				return
			}
			log.Printf("email inbox: lead %s qualified (score=%d)", lead.ID, result.Score)
		}()
	}
}
