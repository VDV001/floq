package notify

import (
	"context"
	"fmt"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramNotifier sends alert messages to a manager's Telegram chat.
type TelegramNotifier struct {
	bot    *tgbotapi.BotAPI
	chatID int64 // manager's telegram chat ID for notifications
}

// NewTelegramNotifier creates a new TelegramNotifier with the given bot token and chat ID.
func NewTelegramNotifier(token string, chatID int64, httpClient *http.Client) (*TelegramNotifier, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	bot, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, httpClient)
	if err != nil {
		return nil, err
	}
	return &TelegramNotifier{bot: bot, chatID: chatID}, nil
}

// SendAlert sends a follow-up reminder notification to the manager's Telegram chat.
func (n *TelegramNotifier) SendAlert(_ context.Context, leadName, company, message string) error {
	text := fmt.Sprintf("Follow-up needed!\n\nLead: %s (%s)\n\n%s", leadName, company, message)
	msg := tgbotapi.NewMessage(n.chatID, text)
	msg.ParseMode = "Markdown"
	_, err := n.bot.Send(msg)
	return err
}
