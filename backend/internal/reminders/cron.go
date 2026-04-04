package reminders

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/leads"
	"github.com/daniil/floq/internal/notify"
)

// Cron periodically checks for stale leads and creates follow-up reminders.
type Cron struct {
	pool      *pgxpool.Pool
	repo      *leads.Repository
	aiClient  *ai.AIClient
	notifier  *notify.TelegramNotifier
	staleDays int
}

// NewCron creates a new Cron instance.
func NewCron(pool *pgxpool.Pool, repo *leads.Repository, aiClient *ai.AIClient, notifier *notify.TelegramNotifier, staleDays int) *Cron {
	return &Cron{
		pool:      pool,
		repo:      repo,
		aiClient:  aiClient,
		notifier:  notifier,
		staleDays: staleDays,
	}
}

// Start runs the reminder cron loop on an hourly ticker.
// It blocks until ctx is cancelled.
func (c *Cron) Start(ctx context.Context) {
	log.Println("Reminders cron started")

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Run once immediately on startup.
	c.check(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("Reminders cron shutting down")
			return
		case <-ticker.C:
			c.check(ctx)
		}
	}
}

// check finds stale leads and creates reminders with AI-generated follow-up messages.
func (c *Cron) check(ctx context.Context) {
	staleLeads, err := c.repo.StaleLeadsWithoutReminder(ctx, c.staleDays)
	if err != nil {
		log.Printf("reminders cron: error querying stale leads: %v", err)
		return
	}

	if len(staleLeads) == 0 {
		return
	}

	log.Printf("reminders cron: found %d stale leads", len(staleLeads))

	for _, lead := range staleLeads {
		// Generate a follow-up message using AI.
		daysAgo := fmt.Sprintf("%d", c.staleDays)
		body, err := c.aiClient.GenerateFollowup(ctx, lead.ContactName, lead.Company, daysAgo, lead.FirstMessage, "")
		if err != nil {
			log.Printf("reminders cron: error generating followup for lead %s: %v", lead.ID, err)
			continue
		}

		// Insert reminder into the database.
		if err := c.repo.CreateReminder(ctx, lead.ID, body); err != nil {
			log.Printf("reminders cron: error creating reminder for lead %s: %v", lead.ID, err)
			continue
		}

		// Send alert to the manager via Telegram.
		if c.notifier != nil {
			if err := c.notifier.SendAlert(lead.ContactName, lead.Company, body); err != nil {
				log.Printf("reminders cron: error sending alert for lead %s: %v", lead.ID, err)
			}
		}

		log.Printf("reminders cron: reminder created for lead %s (%s)", lead.ID, lead.ContactName)
	}
}
