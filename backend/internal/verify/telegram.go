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
