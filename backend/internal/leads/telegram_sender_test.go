package leads

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/stretchr/testify/assert"
)

func TestNewTelegramSender(t *testing.T) {
	s := NewTelegramSender(nil)
	assert.NotNil(t, s)
}

func TestTelegramSender_SendMessage_NoChatID(t *testing.T) {
	s := NewTelegramSender(nil)
	lead := &domain.Lead{ContactName: "Test"}
	err := s.SendMessage(context.Background(), lead, "hello")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no telegram chat ID")
}
