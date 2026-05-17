package inbox

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/daniil/floq/internal/audit"
	auditdomain "github.com/daniil/floq/internal/audit/domain"
	"github.com/daniil/floq/internal/normalize"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

// TelegramBot listens for incoming Telegram messages and creates leads.
type TelegramBot struct {
	bot            *tgbotapi.BotAPI
	repo           LeadRepository
	prospectRepo   ProspectRepository
	aiClient       AIQualifier
	identityLinker IdentityLinker
	logger         *slog.Logger
	ownerID        uuid.UUID
	bookingLink    string
}

// TelegramBotOption configures a *TelegramBot at construction. Used for
// optional dependencies that cross context boundaries (currently the
// IdentityLinker bridge to the leads-context identity store).
type TelegramBotOption func(*TelegramBot)

// WithTelegramIdentityLinker wires the IdentityLinker used to resolve
// and link each newly created Telegram lead to a unified Identity. Pass
// nil (or omit the option) to disable. Linker errors are logged and
// swallowed — the Telegram-side inbound flow never blocks on identity
// backend hiccups.
//
// Named asymmetrically to its email-poller sibling (WithIdentityLinker)
// because the two pollers live in the same package and Go does not
// allow option-name collisions across different option types.
func WithTelegramIdentityLinker(l IdentityLinker) TelegramBotOption {
	return func(b *TelegramBot) { b.identityLinker = l }
}

// WithTelegramLogger overrides the default slog.Logger so the bot
// emits structured warnings to the same handler as the rest of the
// server. Pass nil to keep slog.Default().
func WithTelegramLogger(l *slog.Logger) TelegramBotOption {
	return func(b *TelegramBot) {
		if l != nil {
			b.logger = l
		}
	}
}

// NewTelegramBot creates a new TelegramBot with the given token and dependencies.
func NewTelegramBot(token string, repo LeadRepository, prospectRepo ProspectRepository, aiClient AIQualifier, ownerID uuid.UUID, bookingLink string, httpClient *http.Client, opts ...TelegramBotOption) (*TelegramBot, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	bot, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, httpClient)
	if err != nil {
		return nil, err
	}
	b := &TelegramBot{bot: bot, repo: repo, prospectRepo: prospectRepo, aiClient: aiClient, ownerID: ownerID, bookingLink: bookingLink, logger: slog.Default()}
	for _, opt := range opts {
		opt(b)
	}
	return b, nil
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
		username := normalize.TelegramUsername(msg.From.UserName)
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

		if t.identityLinker != nil && username != "" {
			if err := t.identityLinker.LinkLeadToIdentity(ctx, t.ownerID, lead.ID, "", "", username); err != nil {
				t.logger.WarnContext(ctx, "inbox: identity link failed",
					"lead", lead.ID, "channel", "telegram", "err", err)
			}
		}

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
			qCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			qLeadID := lead.ID
			qCtx = audit.ContextWithCallMeta(qCtx, audit.CallMeta{
				UserID:      lead.UserID,
				LeadID:      &qLeadID,
				RequestType: auditdomain.RequestTypeQualification,
			})
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
