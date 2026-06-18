package inbox

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Compile-time check that *telegramReplyDispatcher satisfies the
// ReplyDispatcher port.
var _ ReplyDispatcher = (*telegramReplyDispatcher)(nil)

// telegramBotSender narrows the Telegram bot API surface to just the
// Send method. The full *tgbotapi.BotAPI satisfies this interface,
// and the test suite can substitute a recorder without standing up
// the real client.
type telegramBotSender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

// inboxMessageWriter narrows LeadRepository to the single CreateMessage
// call the dispatcher makes for outbound history.
type inboxMessageWriter interface {
	CreateMessage(ctx context.Context, msg *InboxMessage) error
}

// telegramReplyDispatcher delivers an approved PendingReply to the
// customer through the Telegram Bot API and records the outbound
// message in the inbox history so the operator UI shows the full
// thread. Currently only the telegram channel is wired; email
// dispatch will land alongside the email auto-reply work that
// extends PendingReplyKind beyond booking_link.
type telegramReplyDispatcher struct {
	bot       telegramBotSender
	targets   ReplyTargetLookup
	inboxRepo inboxMessageWriter
}

// NewTelegramReplyDispatcher builds the telegram reply dispatcher. The bot
// and the inbox message writer are supplied by the composition root; targets
// resolves the lead's chat id without the inbox context importing the leads
// domain.
func NewTelegramReplyDispatcher(bot telegramBotSender, targets ReplyTargetLookup, inboxRepo inboxMessageWriter) ReplyDispatcher {
	return &telegramReplyDispatcher{bot: bot, targets: targets, inboxRepo: inboxRepo}
}

// Dispatch sends the reply via the Telegram Bot API and, only on a
// successful send, writes the outbound message into the inbox history.
// Ordering matters: persisting before sending would risk the UI
// showing a "sent" row for a message that never left the server; the
// reverse risks history loss for a message the customer did receive,
// which we accept as a smaller and more recoverable failure mode.
func (d *telegramReplyDispatcher) Dispatch(ctx context.Context, pr *PendingReply) error {
	if pr.Channel != ChannelTelegram {
		return fmt.Errorf("telegram dispatcher: unsupported channel %q", pr.Channel)
	}
	target, err := d.targets.LookupReplyTarget(ctx, pr.LeadID)
	if err != nil {
		return fmt.Errorf("fetch lead for dispatch: %w", err)
	}
	if target == nil {
		return fmt.Errorf("telegram dispatcher: lead %s not found", pr.LeadID)
	}
	if target.TelegramChatID == nil {
		return fmt.Errorf("telegram dispatcher: lead %s has no telegram_chat_id", pr.LeadID)
	}
	msg := tgbotapi.NewMessage(*target.TelegramChatID, pr.Body)
	if _, err := d.bot.Send(msg); err != nil {
		return fmt.Errorf("telegram send: %w", err)
	}
	outMsg := NewInboxMessage(pr.LeadID, DirectionOutbound, pr.Body)
	if err := d.inboxRepo.CreateMessage(ctx, outMsg); err != nil {
		return fmt.Errorf("persist outbound message: %w", err)
	}
	return nil
}
