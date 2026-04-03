package inbox

import (
	"context"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/leads"
)

// TelegramBot listens for incoming Telegram messages and creates leads.
type TelegramBot struct {
	bot      *tgbotapi.BotAPI
	pool     *pgxpool.Pool
	repo     *leads.Repository
	aiClient *ai.AIClient
	ownerID  uuid.UUID // the manager's user ID who receives all leads
}

// NewTelegramBot creates a new TelegramBot with the given token and dependencies.
func NewTelegramBot(token string, pool *pgxpool.Pool, repo *leads.Repository, aiClient *ai.AIClient, ownerID uuid.UUID) (*TelegramBot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &TelegramBot{bot: bot, pool: pool, repo: repo, aiClient: aiClient, ownerID: ownerID}, nil
}

// Bot returns the underlying BotAPI for sharing with other modules.
func (t *TelegramBot) Bot() *tgbotapi.BotAPI {
	return t.bot
}

// Start begins listening for Telegram updates and processing them.
// It blocks until ctx is cancelled.
func (t *TelegramBot) Start(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := t.bot.GetUpdatesChan(u)

	log.Println("Telegram inbox bot started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Telegram inbox bot shutting down")
			return
		case update := <-updates:
			if update.Message == nil {
				continue
			}
			t.handleMessage(ctx, update.Message)
		}
	}
}

func (t *TelegramBot) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	text := msg.Text
	if text == "" {
		return
	}

	// Build contact name from the sender.
	contactName := msg.From.FirstName
	if msg.From.LastName != "" {
		contactName += " " + msg.From.LastName
	}

	// Check if a lead with this telegram_chat_id already exists.
	existing, err := t.repo.GetLeadByTelegramChatID(ctx, t.ownerID, chatID)
	if err != nil {
		log.Printf("telegram inbox: error looking up lead for chat %d: %v", chatID, err)
		return
	}

	isNewLead := existing == nil
	var lead *leads.Lead

	if isNewLead {
		lead = &leads.Lead{
			ID:             uuid.New(),
			UserID:         t.ownerID,
			Channel:        "telegram",
			ContactName:    contactName,
			Company:        "",
			FirstMessage:   text,
			Status:         "new",
			TelegramChatID: &chatID,
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}
		if err := t.repo.CreateLead(ctx, lead); err != nil {
			log.Printf("telegram inbox: error creating lead: %v", err)
			return
		}
		log.Printf("telegram inbox: new lead created for chat %d (%s)", chatID, contactName)
	} else {
		lead = existing
	}

	// Create inbound message.
	message := &leads.Message{
		ID:        uuid.New(),
		LeadID:    lead.ID,
		Direction: "inbound",
		Body:      text,
		SentAt:    time.Now().UTC(),
	}
	if err := t.repo.CreateMessage(ctx, message); err != nil {
		log.Printf("telegram inbox: error creating message: %v", err)
		return
	}

	// If new lead, trigger async qualification.
	if isNewLead {
		go func() {
			qCtx := context.Background()
			result, err := t.aiClient.Qualify(qCtx, contactName, lead.Channel, lead.FirstMessage)
			if err != nil {
				log.Printf("telegram inbox: qualification error for lead %s: %v", lead.ID, err)
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
				ProviderUsed:      t.aiClient.ProviderName(),
				GeneratedAt:       time.Now().UTC(),
			}
			if err := t.repo.UpsertQualification(qCtx, q); err != nil {
				log.Printf("telegram inbox: error saving qualification for lead %s: %v", lead.ID, err)
				return
			}
			if err := t.repo.UpdateLeadStatus(qCtx, lead.ID, "qualified"); err != nil {
				log.Printf("telegram inbox: error updating lead status for %s: %v", lead.ID, err)
				return
			}
			log.Printf("telegram inbox: lead %s qualified (score=%d)", lead.ID, result.Score)
		}()
	}
}
