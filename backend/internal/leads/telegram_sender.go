package leads

import (
	"context"
	"fmt"

	"github.com/daniil/floq/internal/leads/domain"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramSender implements domain.MessageSender using Telegram Bot API.
type TelegramSender struct {
	bot *tgbotapi.BotAPI
}

// NewTelegramSender creates a new TelegramSender wrapping the given bot.
func NewTelegramSender(bot *tgbotapi.BotAPI) *TelegramSender {
	return &TelegramSender{bot: bot}
}

// SendMessage sends a text message to the lead via Telegram.
func (s *TelegramSender) SendMessage(ctx context.Context, lead *domain.Lead, body string) error {
	if lead.TelegramChatID == nil {
		return fmt.Errorf("lead has no telegram chat ID")
	}
	msg := tgbotapi.NewMessage(*lead.TelegramChatID, body)
	_, err := s.bot.Send(msg)
	return err
}
