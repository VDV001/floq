package inbox

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// NewInboxLead creates a new InboxLead with generated ID, status=new, and timestamps.
func NewInboxLead(userID uuid.UUID, channel Channel, contactName, company, firstMessage string, telegramChatID *int64, emailAddress *string) (*InboxLead, error) {
	if channel != ChannelTelegram && channel != ChannelEmail {
		return nil, fmt.Errorf("invalid channel: %q", channel)
	}
	if contactName == "" {
		return nil, fmt.Errorf("contact name is required")
	}
	now := time.Now().UTC()
	return &InboxLead{
		ID:             uuid.New(),
		UserID:         userID,
		Channel:        channel,
		ContactName:    contactName,
		Company:        company,
		FirstMessage:   firstMessage,
		Status:         StatusNew,
		TelegramChatID: telegramChatID,
		EmailAddress:   emailAddress,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// NewInboxMessage creates a new InboxMessage with generated ID and timestamp.
func NewInboxMessage(leadID uuid.UUID, direction MessageDirection, body string) *InboxMessage {
	return &InboxMessage{
		ID:        uuid.New(),
		LeadID:    leadID,
		Direction: direction,
		Body:      body,
		SentAt:    time.Now().UTC(),
	}
}

// ResolveConfig returns the DB value if non-empty, otherwise the fallback.
func ResolveConfig(dbValue, fallback string) string {
	if dbValue != "" {
		return dbValue
	}
	return fallback
}

// DetectCallAgreement checks if the message indicates the person agrees to a call/meeting.
func DetectCallAgreement(text string) bool {
	lower := strings.ToLower(text)
	markers := []string{
		"давайте созвон", "давай созвон", "готов созвон", "согласен на созвон",
		"можно созвон", "давайте звонок", "давай звонок", "готов к звонку",
		"давайте встреч", "давай встреч", "согласен на встреч", "готов встретить",
		"можем созвон", "можем встретить", "давайте обсудим", "готов обсудить",
		"да, давайте", "да давайте", "конечно, давайте", "с удовольствием",
		"когда удобно", "выберу время", "забронир", "запишусь",
		"да, можно", "да можно", "ок, давай", "ок давай",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}
