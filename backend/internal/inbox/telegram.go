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
	tx                   TxManager
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

// SetTxManager wires the transaction manager that makes new-lead intake
// fail-closed (#206 Part B): CreateLead and the lead.created enqueue commit or
// roll back together. Safe because the receive loop advances the Telegram update
// offset only after handleMessage returns nil, so a rollback re-delivers the
// update on the next poll instead of losing the lead.
func (t *TelegramBot) SetTxManager(tx TxManager) { t.tx = tx }

// updateFetcher is the slice of the Telegram client the receive loop depends on
// (satisfied by *tgbotapi.BotAPI). Depending on the port rather than the
// concrete client keeps the offset-advancement logic unit-testable (#206 Part B).
type updateFetcher interface {
	GetUpdates(tgbotapi.UpdateConfig) ([]tgbotapi.Update, error)
}

// telegramLongPollTimeout is the long-poll wait per GetUpdates call. It also
// bounds how long an idle loop takes to notice ctx cancellation in the worst
// case (an in-flight long poll is abandoned immediately via getUpdatesCtx, so
// shutdown stays responsive regardless).
const telegramLongPollTimeout = 50

// telegramErrBackoff is the pause after a transient GetUpdates error before
// re-polling, so a Telegram outage does not spin the loop.
const telegramErrBackoff = 5 * time.Second

// Start begins listening for Telegram updates and processing them.
// It blocks until ctx is cancelled.
func (t *TelegramBot) Start(ctx context.Context) {
	log.Println("Telegram inbox bot started")
	t.receiveLoop(ctx, t.bot, t.handleMessage)
	log.Println("Telegram inbox bot shutting down")
}

// receiveLoop long-polls Telegram and dispatches each update to handle. Unlike
// the SDK's GetUpdatesChan (which advances the offset as soon as it buffers an
// update), this loop advances the offset only AFTER handle returns nil, so a
// failed intake leaves the update unconfirmed and the next poll re-delivers it
// (#206 Part B fail-closed). A handler error halts the rest of the batch and
// re-polls from the failed update's id; non-message updates advance without
// dispatch.
func (t *TelegramBot) receiveLoop(ctx context.Context, fetcher updateFetcher, handle func(context.Context, *tgbotapi.Message) error) {
	offset := 0
	for {
		if ctx.Err() != nil {
			return
		}
		cfg := tgbotapi.NewUpdate(offset)
		cfg.Timeout = telegramLongPollTimeout
		updates, err := getUpdatesCtx(ctx, fetcher, cfg)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("telegram inbox: get updates error: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(telegramErrBackoff):
			}
			continue
		}
		for _, update := range updates {
			if update.Message == nil {
				offset = update.UpdateID + 1
				continue
			}
			if err := handle(ctx, update.Message); err != nil {
				// Transient: do NOT advance past this update. Break the batch so
				// the next poll re-requests from this id and re-delivers it.
				log.Printf("telegram inbox: intake failed for update %d, will retry: %v", update.UpdateID, err)
				break
			}
			offset = update.UpdateID + 1
		}
	}
}

// getUpdatesCtx runs the blocking GetUpdates long poll in a goroutine and
// returns as soon as ctx is cancelled, so shutdown is not stalled for up to the
// long-poll timeout. An abandoned in-flight poll completes harmlessly in the
// background (its result is discarded via the buffered channel) and, because its
// updates were never confirmed by a higher offset, they are re-delivered on the
// next process start.
func getUpdatesCtx(ctx context.Context, fetcher updateFetcher, cfg tgbotapi.UpdateConfig) ([]tgbotapi.Update, error) {
	type result struct {
		updates []tgbotapi.Update
		err     error
	}
	ch := make(chan result, 1)
	go func() {
		up, err := fetcher.GetUpdates(cfg)
		ch <- result{up, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.updates, r.err
	}
}

// handleMessage processes one inbound Telegram message. It returns a non-nil
// error only for transient failures where the caller must NOT advance the update
// offset, so the next poll re-delivers the update (#206 Part B fail-closed).
// Permanent/skip conditions (non-text, invalid entity, swallowed best-effort
// side-effects) return nil so the offset advances and the update is not
// re-processed forever.
func (t *TelegramBot) handleMessage(ctx context.Context, msg *tgbotapi.Message) error {
	chatID := msg.Chat.ID
	text := msg.Text
	if text == "" {
		return nil
	}

	// Build contact name from the sender.
	contactName := msg.From.FirstName
	if msg.From.LastName != "" {
		contactName += " " + msg.From.LastName
	}

	// Check if a lead with this telegram_chat_id already exists.
	existing, err := t.repo.GetLeadByTelegramChatID(ctx, t.ownerID, chatID)
	if err != nil {
		// Transient: signal retry rather than dropping the update (#206 Part B).
		return fmt.Errorf("look up lead for chat %d: %w", chatID, err)
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
			return nil
		}
		lead = newLead
		if prospect != nil {
			lead.SourceID = prospect.SourceID
		}
		// #206 Part B: persist the lead and enqueue lead.created atomically. A
		// failed enqueue rolls the lead back and the error propagates so the
		// receive loop leaves the update offset un-advanced for re-delivery
		// (fail-closed). Safe because the offset advances only after a nil return.
		if err := t.commitLeadIntake(ctx, lead); err != nil {
			log.Printf("telegram inbox: error creating lead for chat %d, will retry: %v", chatID, err)
			return err
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
		return nil
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
	return nil
}

// commitLeadIntake persists a new inbound lead and enqueues its lead.created
// event. When a transaction manager and emitter are both wired (webhooks
// enabled), the two run inside one WithTx so they commit or roll back together —
// a failed enqueue undoes the lead and returns an error (fail-closed, #206 Part
// B). Otherwise it falls back to a plain create with a best-effort post-commit
// emit, preserving the zero-overhead path when webhooks are disabled.
func (t *TelegramBot) commitLeadIntake(ctx context.Context, lead *InboxLead) error {
	if t.tx != nil && t.leadCreatedEmitter != nil {
		return t.tx.WithTx(ctx, func(txCtx context.Context) error {
			if err := t.repo.CreateLead(txCtx, lead); err != nil {
				return err
			}
			return t.leadCreatedEmitter.EmitLeadCreated(txCtx, lead)
		})
	}
	if err := t.repo.CreateLead(ctx, lead); err != nil {
		return err
	}
	if t.leadCreatedEmitter != nil {
		if err := t.leadCreatedEmitter.EmitLeadCreated(ctx, lead); err != nil {
			log.Printf("telegram inbox: lead.created emit failed (best-effort): %v", err)
		}
	}
	return nil
}
