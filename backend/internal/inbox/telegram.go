package inbox

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	auditdomain "github.com/daniil/floq/internal/audit/domain"
	"github.com/daniil/floq/internal/normalize"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

// bookingLinkReplyTemplate is the user-facing body the HITL queue
// enqueues when DetectCallAgreement triggers. Kept as a single
// package-level constant so any future copy change is one edit,
// not a hunt through handleMessage. The %s placeholder is the
// configured booking URL — never interpolate untrusted input here.
const bookingLinkReplyTemplate = "Отлично! Вот ссылка для выбора удобного времени для звонка: %s\n\nВыберите слот и я получу уведомление. До связи!"

// TelegramBot listens for incoming Telegram messages and creates leads.
type TelegramBot struct {
	bot                   *tgbotapi.BotAPI
	repo                  LeadRepository
	prospectRepo          ProspectRepository
	aiClient              AIQualifier
	identityLinker        IdentityLinker
	pendingProposer       PendingReplyProposer
	logger                *slog.Logger
	ownerID               uuid.UUID
	bookingLink           string
	leadCreatedEmitter   LeadCreatedEmitter
	leadQualifiedEmitter LeadQualifiedEmitter
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

// WithTelegramPendingReplyProposer wires the HITL queue used by the
// booking-link branch in handleMessage. When set, DetectCallAgreement
// enqueues a pending reply for operator approval instead of sending
// the booking URL directly. When nil (or omitted), the booking-link
// branch is suppressed entirely — a secure default that never falls
// back to instant send.
//
// Named asymmetrically to its email-poller sibling for the same
// reason as WithTelegramIdentityLinker — Go does not allow option-name
// collisions across different option types in the same package.
func WithTelegramPendingReplyProposer(p PendingReplyProposer) TelegramBotOption {
	return func(b *TelegramBot) { b.pendingProposer = p }
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

// SetPendingProposer wires the HITL queue after construction. Used by
// the composition root to break the
// bot -> usecase -> dispatcher -> bot dependency cycle: the usecase
// needs a dispatcher built from tgBot.Bot(), and the bot needs the
// resulting usecase as proposer. Mirrors the existing leadsUC
// .SetSender pattern in main.go.
func (t *TelegramBot) SetPendingProposer(p PendingReplyProposer) {
	t.pendingProposer = p
}

// SetLeadCreatedEmitter wires the best-effort post-commit lead.created emitter
// (#199 / #206). The Telegram update offset is already advanced by the time a
// lead is written, so intake cannot be fail-closed without losing the lead — the
// event is emitted post-commit, at-most-once, NOT inside a transaction.
func (t *TelegramBot) SetLeadCreatedEmitter(e LeadCreatedEmitter) { t.leadCreatedEmitter = e }

// SetLeadQualifiedEmitter wires the best-effort post-commit lead.qualified
// emitter for the inbox auto-qualification path (#199 / #206). One-shot goroutine
// with no retry, so it is post-commit best-effort, not transactional.
func (t *TelegramBot) SetLeadQualifiedEmitter(e LeadQualifiedEmitter) { t.leadQualifiedEmitter = e }

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
		// lead.created is emitted best-effort post-commit, NOT in a transaction:
		// the Telegram update offset is already advanced, so a fail-closed
		// rollback would lose the lead with no retry (#206). At-most-once, as
		// before #199.
		if t.leadCreatedEmitter != nil {
			if err := t.leadCreatedEmitter.EmitLeadCreated(ctx, lead); err != nil {
				log.Printf("telegram inbox: lead.created emit failed (best-effort): %v", err)
			}
		}

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
		// Re-engagement: a new inbound message on an archived lead resurfaces
		// it so the operator sees the reply in the inbox feed again. Without
		// this the message would attach to a hidden lead and be silently lost.
		if existing.ArchivedAt != nil {
			if err := t.repo.UnarchiveLead(ctx, lead.ID); err != nil {
				log.Printf("telegram inbox: error unarchiving lead %s on re-engagement: %v", lead.ID, err)
			} else {
				lead.ArchivedAt = nil
			}
		}
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

	// HITL gate for booking link: when DetectCallAgreement triggers,
	// the bot enqueues a pending reply for operator approval rather
	// than sending the calendar URL directly. A misfired detector
	// would otherwise leak a booking link to a lead who never asked,
	// so the secure default when no proposer is wired is to suppress
	// the branch entirely — we do NOT fall back to instant send.
	if DetectCallAgreement(text) {
		switch {
		case t.pendingProposer == nil:
			t.logger.WarnContext(ctx, "booking link suppressed: no pending reply proposer wired",
				slog.Int64("chat_id", chatID), slog.String("lead_id", lead.ID.String()))
		case t.bookingLink == "":
			// Enqueueing with an empty URL would let an operator
			// approve a customer-visible message ending in
			// "…ссылка для звонка: " with nothing after it.
			// Suppress the branch instead — operator can write the
			// message manually if needed.
			t.logger.WarnContext(ctx, "booking link suppressed: bookingLink not configured",
				slog.Int64("chat_id", chatID), slog.String("lead_id", lead.ID.String()))
		default:
			bookingMsg := fmt.Sprintf(bookingLinkReplyTemplate, t.bookingLink)
			if _, err := t.pendingProposer.Propose(ctx, lead.UserID, lead.ID, ChannelTelegram, PendingReplyKindBookingLink, bookingMsg, text); err != nil {
				t.logger.WarnContext(ctx, "failed to enqueue booking reply for approval",
					slog.String("lead_id", lead.ID.String()), slog.Any("error", err))
			}
		}
	}

	// Trigger async qualification on every inbound message (re-qualifies with latest context).
	{
		qualifyText := text // use latest message for qualification
		go func() {
			qCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			qLeadID := lead.ID
			qCtx = auditdomain.ContextWithCallMeta(qCtx, auditdomain.CallMeta{
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
			// lead.qualified is emitted best-effort post-commit, NOT in a
			// transaction: this is a one-shot goroutine with no retry, so a
			// fail-closed rollback would silently drop the qualification (#206).
			if t.leadQualifiedEmitter != nil {
				lead.Status = StatusQualified
				if err := t.leadQualifiedEmitter.EmitLeadQualified(qCtx, lead); err != nil {
					log.Printf("telegram inbox: lead.qualified emit failed (best-effort): %v", err)
				}
			}
		}()
	}
}
