package main

import (
	"context"
	"fmt"

	"github.com/daniil/floq/internal/inbox"
	leadsdomain "github.com/daniil/floq/internal/leads/domain"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

// Compile-time check that *telegramReplyDispatcher satisfies the inbox
// ReplyDispatcher port.
var _ inbox.ReplyDispatcher = (*telegramReplyDispatcher)(nil)

// telegramBotSender narrows the Telegram bot API surface to just the
// Send method. The full *tgbotapi.BotAPI satisfies this interface,
// and the test suite can substitute a recorder without standing up
// the real client.
type telegramBotSender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

// leadChatIDFetcher narrows leadsdomain.Repository to the single
// lookup the dispatcher needs (the lead's TelegramChatID, which is
// stored on Lead, not derivable from a pending reply alone).
type leadChatIDFetcher interface {
	GetLead(ctx context.Context, id uuid.UUID) (*leadsdomain.Lead, error)
}

// inboxMessageWriter narrows inbox.LeadRepository to the single
// CreateMessage call the dispatcher makes for outbound history.
type inboxMessageWriter interface {
	CreateMessage(ctx context.Context, msg *inbox.InboxMessage) error
}

// telegramReplyDispatcher delivers an approved PendingReply to the
// customer through the Telegram Bot API and records the outbound
// message in the inbox history so the operator UI shows the full
// thread. Currently only the telegram channel is wired; email
// dispatch will land alongside the email auto-reply work that
// extends PendingReplyKind beyond booking_link.
type telegramReplyDispatcher struct {
	bot       telegramBotSender
	leads     leadChatIDFetcher
	inboxRepo inboxMessageWriter
}

func newTelegramReplyDispatcher(bot telegramBotSender, leads leadChatIDFetcher, inboxRepo inboxMessageWriter) *telegramReplyDispatcher {
	return &telegramReplyDispatcher{bot: bot, leads: leads, inboxRepo: inboxRepo}
}

// Dispatch sends the reply via the Telegram Bot API and, only on a
// successful send, writes the outbound message into the inbox history.
// Ordering matters: persisting before sending would risk the UI
// showing a "sent" row for a message that never left the server; the
// reverse risks history loss for a message the customer did receive,
// which we accept as a smaller and more recoverable failure mode.
func (d *telegramReplyDispatcher) Dispatch(ctx context.Context, pr *inbox.PendingReply) error {
	if pr.Channel != inbox.ChannelTelegram {
		return fmt.Errorf("telegram dispatcher: unsupported channel %q", pr.Channel)
	}
	lead, err := d.leads.GetLead(ctx, pr.LeadID)
	if err != nil {
		return fmt.Errorf("fetch lead for dispatch: %w", err)
	}
	if lead == nil {
		return fmt.Errorf("telegram dispatcher: lead %s not found", pr.LeadID)
	}
	if lead.TelegramChatID == nil {
		return fmt.Errorf("telegram dispatcher: lead %s has no telegram_chat_id", pr.LeadID)
	}
	msg := tgbotapi.NewMessage(*lead.TelegramChatID, pr.Body)
	if _, err := d.bot.Send(msg); err != nil {
		return fmt.Errorf("telegram send: %w", err)
	}
	outMsg := inbox.NewInboxMessage(pr.LeadID, inbox.DirectionOutbound, pr.Body)
	if err := d.inboxRepo.CreateMessage(ctx, outMsg); err != nil {
		return fmt.Errorf("persist outbound message: %w", err)
	}
	return nil
}
