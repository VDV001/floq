package leads

import (
	"testing"
	"time"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestLeadToResponse(t *testing.T) {
	email := "test@example.com"
	chatID := int64(123)
	sourceID := uuid.New()
	now := time.Now().UTC()
	item := domain.LeadWithSource{
		Lead: domain.Lead{
			ID:             uuid.New(),
			UserID:         uuid.New(),
			Channel:        domain.ChannelTelegram,
			ContactName:    "Alice",
			Company:        "Acme",
			FirstMessage:   "Hello",
			Status:         domain.StatusNew,
			TelegramChatID: &chatID,
			EmailAddress:   &email,
			SourceID:       &sourceID,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		SourceName: "Website",
	}

	resp := LeadWithSourceToResponse(&item)

	assert.Equal(t, item.ID, resp.ID)
	assert.Equal(t, item.UserID, resp.UserID)
	assert.Equal(t, "telegram", resp.Channel)
	assert.Equal(t, "Alice", resp.ContactName)
	assert.Equal(t, "Acme", resp.Company)
	assert.Equal(t, "Hello", resp.FirstMessage)
	assert.Equal(t, "new", resp.Status)
	assert.Equal(t, &chatID, resp.TelegramChatID)
	assert.Equal(t, &email, resp.EmailAddress)
	assert.Equal(t, &sourceID, resp.SourceID)
	assert.Equal(t, "Website", resp.SourceName)
	assert.Equal(t, now, resp.CreatedAt)
	assert.Equal(t, now, resp.UpdatedAt)
}

func TestLeadToResponse_NilOptionals(t *testing.T) {
	lead := &domain.Lead{
		ID:          uuid.New(),
		UserID:      uuid.New(),
		Channel:     domain.ChannelEmail,
		ContactName: "Bob",
		Status:      domain.StatusQualified,
	}

	resp := LeadToResponse(lead)
	assert.Nil(t, resp.TelegramChatID)
	assert.Nil(t, resp.EmailAddress)
	assert.Nil(t, resp.SourceID)
}

func TestLeadsToResponse(t *testing.T) {
	leads := []domain.LeadWithSource{
		{Lead: domain.Lead{ID: uuid.New(), ContactName: "A", Channel: domain.ChannelEmail, Status: domain.StatusNew}},
		{Lead: domain.Lead{ID: uuid.New(), ContactName: "B", Channel: domain.ChannelTelegram, Status: domain.StatusClosed}},
	}

	resp := LeadsToResponse(leads)
	assert.Len(t, resp, 2)
	assert.Equal(t, "A", resp[0].ContactName)
	assert.Equal(t, "B", resp[1].ContactName)
}

func TestLeadsToResponse_Empty(t *testing.T) {
	resp := LeadsToResponse([]domain.LeadWithSource{})
	assert.Len(t, resp, 0)
}

func TestMessageToResponse(t *testing.T) {
	now := time.Now().UTC()
	msg := &domain.Message{
		ID:        uuid.New(),
		LeadID:    uuid.New(),
		Direction: domain.DirectionOutbound,
		Body:      "test body",
		SentAt:    now,
	}

	resp := MessageToResponse(msg)
	assert.Equal(t, msg.ID, resp.ID)
	assert.Equal(t, msg.LeadID, resp.LeadID)
	assert.Equal(t, "outbound", resp.Direction)
	assert.Equal(t, "test body", resp.Body)
	assert.Equal(t, now, resp.SentAt)
}

func TestMessagesToResponse(t *testing.T) {
	msgs := []domain.Message{
		{ID: uuid.New(), Direction: domain.DirectionInbound, Body: "a"},
		{ID: uuid.New(), Direction: domain.DirectionOutbound, Body: "b"},
	}
	resp := MessagesToResponse(msgs)
	assert.Len(t, resp, 2)
	assert.Equal(t, "inbound", resp[0].Direction)
	assert.Equal(t, "outbound", resp[1].Direction)
}

func TestQualificationToResponse(t *testing.T) {
	now := time.Now().UTC()
	q := &domain.Qualification{
		ID:                uuid.New(),
		LeadID:            uuid.New(),
		IdentifiedNeed:    "CRM system",
		EstimatedBudget:   "50000 RUB",
		Deadline:          "2 weeks",
		Score:             85,
		ScoreReason:       "High intent",
		RecommendedAction: "Schedule demo",
		ProviderUsed:      "openai",
		GeneratedAt:       now,
	}

	resp := QualificationToResponse(q)
	assert.Equal(t, q.ID, resp.ID)
	assert.Equal(t, q.LeadID, resp.LeadID)
	assert.Equal(t, "CRM system", resp.IdentifiedNeed)
	assert.Equal(t, "50000 RUB", resp.EstimatedBudget)
	assert.Equal(t, "2 weeks", resp.Deadline)
	assert.Equal(t, 85, resp.Score)
	assert.Equal(t, "High intent", resp.ScoreReason)
	assert.Equal(t, "Schedule demo", resp.RecommendedAction)
	assert.Equal(t, "openai", resp.ProviderUsed)
	assert.Equal(t, now, resp.GeneratedAt)
}

func TestDraftToResponse(t *testing.T) {
	now := time.Now().UTC()
	d := &domain.Draft{
		ID:        uuid.New(),
		LeadID:    uuid.New(),
		Body:      "Dear customer...",
		CreatedAt: now,
	}

	resp := DraftToResponse(d)
	assert.Equal(t, d.ID, resp.ID)
	assert.Equal(t, d.LeadID, resp.LeadID)
	assert.Equal(t, "Dear customer...", resp.Body)
	assert.Equal(t, now, resp.CreatedAt)
}
