package outbound

import (
	"context"

	"github.com/daniil/floq/internal/tgclient"
)

// Compile-time check.
var _ TelegramMessenger = (*MTProtoMessenger)(nil)

// MTProtoMessenger sends Telegram messages via gotd/td MTProto client.
type MTProtoMessenger struct{}

func NewMTProtoMessenger() *MTProtoMessenger {
	return &MTProtoMessenger{}
}

func (m *MTProtoMessenger) SendMessage(ctx context.Context, sessionData []byte, target, body string) error {
	client := tgclient.NewClient()
	client.LoadSession(sessionData)
	return client.SendMessage(ctx, target, body)
}
