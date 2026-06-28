package inbox

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"time"

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
	qualJobs              QualificationJobEnqueuer
	identityLinker        IdentityLinker
	pendingProposer       PendingReplyProposer
	logger                *slog.Logger
	ownerID               uuid.UUID
	bookingLink           string
	leadCreatedEmitter LeadCreatedEmitter
	tx                 TxManager
	retries            *retryTracker
	onQuarantine       func(channel string)
	errBackoff           time.Duration
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
func NewTelegramBot(token string, repo LeadRepository, prospectRepo ProspectRepository, ownerID uuid.UUID, bookingLink string, httpClient *http.Client, opts ...TelegramBotOption) (*TelegramBot, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	bot, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, httpClient)
	if err != nil {
		return nil, err
	}
	b := &TelegramBot{bot: bot, repo: repo, prospectRepo: prospectRepo, ownerID: ownerID, bookingLink: bookingLink, logger: slog.Default(), retries: newRetryTracker(defaultIntakeMaxAttempts), onQuarantine: func(string) {}, errBackoff: telegramErrBackoff}
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

// SetTxManager wires the transaction manager that makes new-lead intake
// fail-closed (#206 Part B): CreateLead and the lead.created enqueue commit or
// roll back together. Safe because the receive loop advances the Telegram update
// offset only after handleMessage returns nil, so a rollback re-delivers the
// update on the next poll instead of losing the lead.
func (t *TelegramBot) SetTxManager(tx TxManager) { t.tx = tx }

// SetIntakeRetryCap overrides the number of consecutive failed intake attempts
// (per source update_id) tolerated before a poison update is quarantined. A
// non-positive cap disables quarantine (fail-closed forever — the pre-#208
// behaviour). Wired from config at the composition root (#208).
func (t *TelegramBot) SetIntakeRetryCap(maxAttempts int) {
	t.retries = newRetryTracker(maxAttempts)
}

// SetQuarantineObserver wires the callback fired once when an update is
// quarantined (retry cap reached), so the composition root can publish a metric
// without this package importing the metrics package (#208).
func (t *TelegramBot) SetQuarantineObserver(fn func(channel string)) {
	if fn != nil {
		t.onQuarantine = fn
	}
}

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
			if !t.backoff(ctx) {
				return
			}
			continue
		}
		for _, update := range updates {
			if update.Message == nil {
				offset = update.UpdateID + 1
				continue
			}
			if err := handle(ctx, update.Message); err != nil {
				key := strconv.Itoa(update.UpdateID)
				attempts, exhausted := t.retries.fail(key)
				if exhausted {
					// Retry cap reached (#208): a deterministic failure would
					// otherwise re-deliver this update forever. Quarantine —
					// advance past it and alert loudly. The message stays in the
					// Telegram chat history for manual recovery.
					log.Printf("telegram inbox: intake quarantined for update %d after %d attempts, skipping: %v", update.UpdateID, attempts, err)
					t.onQuarantine("telegram")
					offset = update.UpdateID + 1
					continue
				}
				// Transient: do NOT advance past this update. Back off (a stuck
				// update would otherwise be re-delivered instantly, busy-looping
				// the DB and Telegram API), then break so the next poll
				// re-requests from this id and re-delivers it.
				log.Printf("telegram inbox: intake failed for update %d (attempt %d), will retry: %v", update.UpdateID, attempts, err)
				if !t.backoff(ctx) {
					return
				}
				break
			}
			t.retries.succeed(strconv.Itoa(update.UpdateID))
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

// backoff pauses for t.errBackoff, returning false if ctx is cancelled during
// the wait (the caller should then exit). A zero errBackoff returns immediately
// — used by unit tests to keep the loop fast.
func (t *TelegramBot) backoff(ctx context.Context) bool {
	if t.errBackoff <= 0 {
		return ctx.Err() == nil
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(t.errBackoff):
		return true
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
		// Build the durable auto-qualification job, enqueued atomically with the
		// lead below so a retry never loses the qualification (#206 Part C).
		job := t.newQualificationJob(ctx, lead, contactName, text)
		// #206 Part B/C: persist the lead, enqueue lead.created, and enqueue the
		// qualification job atomically. A failure rolls all back and propagates so
		// the receive loop leaves the update offset un-advanced for re-delivery
		// (fail-closed). Safe because the offset advances only after a nil return.
		if err := t.commitLeadIntake(ctx, lead, job); err != nil {
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

	// Re-qualify an existing lead on each new message (its score may change with
	// new context). New leads were already enqueued atomically with the lead in
	// commitLeadIntake; this re-qualification is best-effort — a failed enqueue
	// is logged but does not gate the offset (the lead is already durable and
	// re-running handleMessage would duplicate the inbound message).
	if !isNewLead {
		if job := t.newQualificationJob(ctx, lead, contactName, text); job != nil {
			if err := t.qualJobs.EnqueueQualificationJob(ctx, job); err != nil {
				log.Printf("telegram inbox: re-qualification enqueue failed (best-effort) for lead %s: %v", lead.ID, err)
			}
		}
	}
	return nil
}

// newQualificationJob builds the durable auto-qualification job for a Telegram
// lead, capturing the message text as the qualifier input. Returns nil when no
// enqueuer is wired or the text is empty — qualification is simply skipped.
func (t *TelegramBot) newQualificationJob(ctx context.Context, lead *InboxLead, contactName, text string) *QualificationJob {
	if t.qualJobs == nil {
		return nil
	}
	job, err := NewQualificationJob(lead.ID, lead.UserID, contactName, lead.Channel, text)
	if err != nil {
		t.logger.WarnContext(ctx, "inbox: skip qualification job", "lead", lead.ID, "err", err)
		return nil
	}
	return job
}

// SetQualificationEnqueuer wires the durable qualification queue. When set, every
// new lead enqueues a job (atomically with the lead) and each subsequent message
// re-enqueues one (best-effort) for the qualification worker to score.
func (t *TelegramBot) SetQualificationEnqueuer(q QualificationJobEnqueuer) { t.qualJobs = q }

// commitLeadIntake persists a new inbound lead and, atomically with it, enqueues
// its lead.created event and the auto-qualification job. When a transaction
// manager is wired (production), all three run inside one WithTx so they commit
// or roll back together — a failure undoes the lead and returns an error
// (fail-closed, #206 Part B/C), and the receive loop leaves the update offset
// un-advanced for re-delivery. Without a tx (tests), it falls back to a plain
// create with best-effort post-commit side-effects. job may be nil.
func (t *TelegramBot) commitLeadIntake(ctx context.Context, lead *InboxLead, job *QualificationJob) error {
	if t.tx != nil && (t.leadCreatedEmitter != nil || job != nil) {
		return t.tx.WithTx(ctx, func(txCtx context.Context) error {
			if err := t.repo.CreateLead(txCtx, lead); err != nil {
				return err
			}
			if t.leadCreatedEmitter != nil {
				if err := t.leadCreatedEmitter.EmitLeadCreated(txCtx, lead); err != nil {
					return err
				}
			}
			if job != nil {
				if err := t.qualJobs.EnqueueQualificationJob(txCtx, job); err != nil {
					return err
				}
			}
			return nil
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
	if job != nil {
		if err := t.qualJobs.EnqueueQualificationJob(ctx, job); err != nil {
			log.Printf("telegram inbox: qualification enqueue failed (best-effort): %v", err)
		}
	}
	return nil
}
