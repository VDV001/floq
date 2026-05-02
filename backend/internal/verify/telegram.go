package verify

import (
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramResult struct {
	Username string `json:"username"`
	Exists   bool   `json:"exists"`
	Error    string `json:"error,omitempty"`
}

// BotTelegramVerifier adapts the tgbotapi SDK to the TelegramVerifier
// interface owned by usecase.go. usecase.go itself does not import
// tgbotapi — only this file does.
type BotTelegramVerifier struct {
	bot *tgbotapi.BotAPI
}

// NewBotTelegramVerifier returns a TelegramVerifier interface (not the
// concrete *BotTelegramVerifier) so a nil bot collapses to a true-nil
// interface value. Returning a typed-nil pointer would have produced a
// non-nil interface that still satisfies TelegramVerifier — and the
// usecase's `if uc.telegram != nil` guard would let it through, only to
// panic on the nil-receiver Verify call.
func NewBotTelegramVerifier(bot *tgbotapi.BotAPI) TelegramVerifier {
	if bot == nil {
		return nil
	}
	return &BotTelegramVerifier{bot: bot}
}

// Verify implements TelegramVerifier by delegating to the package-level
// VerifyTelegram function.
func (b *BotTelegramVerifier) Verify(username string) TelegramResult {
	return VerifyTelegram(b.bot, username)
}

func VerifyTelegram(bot *tgbotapi.BotAPI, username string) TelegramResult {
	username = strings.TrimSpace(username)
	username = strings.TrimLeft(username, "@")

	if username == "" {
		return TelegramResult{
			Username: "",
			Exists:   false,
			Error:    "empty username",
		}
	}

	chatCfg := tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{
			SuperGroupUsername: "@" + username,
		},
	}

	_, err := bot.GetChat(chatCfg)
	if err != nil {
		return TelegramResult{
			Username: username,
			Exists:   false,
			Error:    err.Error(),
		}
	}

	return TelegramResult{
		Username: username,
		Exists:   true,
	}
}
