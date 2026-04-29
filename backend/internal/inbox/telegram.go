package inbox

import (
	"context"
	"log"
	"net/http"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

// TelegramBot listens for incoming Telegram messages and creates leads.
type TelegramBot struct {
	bot          *tgbotapi.BotAPI
	repo         LeadRepository
	prospectRepo ProspectRepository
	aiClient     AIQualifier
	ownerID      uuid.UUID
	bookingLink  string
}

// NewTelegramBot creates a new TelegramBot with the given token and dependencies.
func NewTelegramBot(token string, repo LeadRepository, prospectRepo ProspectRepository, aiClient AIQualifier, ownerID uuid.UUID, bookingLink string, httpClient *http.Client) (*TelegramBot, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	bot, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, httpClient)
	if err != nil {
		return nil, err
	}
	return &TelegramBot{bot: bot, repo: repo, prospectRepo: prospectRepo, aiClient: aiClient, ownerID: ownerID, bookingLink: bookingLink}, nil
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
	var lead *InboxLead

	if isNewLead {
		company := ""
		var prospect *ProspectMatch
		username := msg.From.UserName
		if username != "" && t.prospectRepo != nil {
			p, pErr := t.prospectRepo.FindByTelegramUsername(ctx, t.ownerID, username)
			if pErr == nil && p != nil && p.Status != ProspectStatusConverted {
				prospect = p
				if p.Name != "" {
					contactName = p.Name
				}
				company = p.Company
			}
		}

		newLead, err := NewInboxLead(t.ownerID, ChannelTelegram, contactName, company, text, &chatID, nil)
		if err != nil {
			log.Printf("telegram inbox: error creating lead entity: %v", err)
			return
		}
		lead = newLead
		if prospect != nil {
			lead.SourceID = prospect.SourceID
		}
		if err := t.repo.CreateLead(ctx, lead); err != nil {
			log.Printf("telegram inbox: error creating lead: %v", err)
			return
		}
		log.Printf("telegram inbox: new lead created for chat %d (%s)", chatID, contactName)

		if prospect != nil {
			if convErr := t.prospectRepo.ConvertToLead(ctx, prospect.ID, lead.ID); convErr != nil {
				log.Printf("telegram inbox: error converting prospect %s: %v", prospect.ID, convErr)
			} else {
				log.Printf("telegram inbox: prospect %s auto-converted to lead %s", prospect.ID, lead.ID)
			}
		}
	} else {
		lead = existing
		// Update first_message if current one is trivial (/start, привет, etc.)
		if len(lead.FirstMessage) < 20 && len(text) > 20 {
			t.repo.UpdateFirstMessage(ctx, lead.ID, text)
			lead.FirstMessage = text
		}
	}

	// Create inbound message.
	message := NewInboxMessage(lead.ID, DirectionInbound, text)
	if err := t.repo.CreateMessage(ctx, message); err != nil {
		log.Printf("telegram inbox: error creating message: %v", err)
		return
	}

	// Auto-reply with booking link if lead agrees to a call
	if DetectCallAgreement(text) {
		bookingMsg := "Отлично! Вот ссылка для выбора удобного времени для звонка: " + t.bookingLink + "\n\nВыберите слот и я получу уведомление. До связи!"
		tgReply := tgbotapi.NewMessage(chatID, bookingMsg)
		if _, err := t.bot.Send(tgReply); err != nil {
			log.Printf("telegram inbox: error sending booking link: %v", err)
		} else {
			// Save as outbound message
			outMsg := NewInboxMessage(lead.ID, DirectionOutbound, bookingMsg)
			t.repo.CreateMessage(ctx, outMsg)
			log.Printf("telegram inbox: sent booking link to chat %d", chatID)
		}
	}

	// Trigger async qualification on every inbound message (re-qualifies with latest context).
	{
		qualifyText := text // use latest message for qualification
		go func() {
			qCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			result, err := t.aiClient.Qualify(qCtx, contactName, string(lead.Channel), qualifyText)
			if err != nil {
				log.Printf("telegram inbox: qualification error for lead %s: %v", lead.ID, err)
				return
			}

			q := &InboxQualification{
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
			if err := t.repo.UpdateLeadStatus(qCtx, lead.ID, StatusQualified); err != nil {
				log.Printf("telegram inbox: error updating lead status for %s: %v", lead.ID, err)
				return
			}
			log.Printf("telegram inbox: lead %s qualified (score=%d)", lead.ID, result.Score)
		}()
	}
}
