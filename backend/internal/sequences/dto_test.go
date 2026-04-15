package sequences

import (
	"testing"
	"time"

	"github.com/daniil/floq/internal/sequences/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestSequenceToResponse(t *testing.T) {
	now := time.Now().UTC()
	s := &domain.Sequence{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Name:      "Test Seq",
		IsActive:  true,
		CreatedAt: now,
	}

	resp := SequenceToResponse(s)
	assert.Equal(t, s.ID, resp.ID)
	assert.Equal(t, s.UserID, resp.UserID)
	assert.Equal(t, s.Name, resp.Name)
	assert.True(t, resp.IsActive)
	assert.Equal(t, now, resp.CreatedAt)
}

func TestSequencesToResponse(t *testing.T) {
	seqs := []domain.Sequence{
		{ID: uuid.New(), Name: "Seq1"},
		{ID: uuid.New(), Name: "Seq2"},
		{ID: uuid.New(), Name: "Seq3"},
	}

	resp := SequencesToResponse(seqs)
	assert.Len(t, resp, 3)
	assert.Equal(t, "Seq1", resp[0].Name)
	assert.Equal(t, "Seq2", resp[1].Name)
	assert.Equal(t, "Seq3", resp[2].Name)
}

func TestSequencesToResponse_Empty(t *testing.T) {
	resp := SequencesToResponse([]domain.Sequence{})
	assert.Len(t, resp, 0)
	assert.NotNil(t, resp)
}

func TestStepToResponse(t *testing.T) {
	now := time.Now().UTC()
	s := &domain.SequenceStep{
		ID:         uuid.New(),
		SequenceID: uuid.New(),
		StepOrder:  2,
		DelayDays:  3,
		PromptHint: "follow up",
		Channel:    domain.StepChannelTelegram,
		CreatedAt:  now,
	}

	resp := StepToResponse(s)
	assert.Equal(t, s.ID, resp.ID)
	assert.Equal(t, s.SequenceID, resp.SequenceID)
	assert.Equal(t, 2, resp.StepOrder)
	assert.Equal(t, 3, resp.DelayDays)
	assert.Equal(t, "follow up", resp.PromptHint)
	assert.Equal(t, "telegram", resp.Channel)
	assert.Equal(t, now, resp.CreatedAt)
}

func TestStepsToResponse(t *testing.T) {
	steps := []domain.SequenceStep{
		{ID: uuid.New(), StepOrder: 1, Channel: domain.StepChannelEmail},
		{ID: uuid.New(), StepOrder: 2, Channel: domain.StepChannelTelegram},
	}

	resp := StepsToResponse(steps)
	assert.Len(t, resp, 2)
	assert.Equal(t, 1, resp[0].StepOrder)
	assert.Equal(t, "email", resp[0].Channel)
	assert.Equal(t, 2, resp[1].StepOrder)
	assert.Equal(t, "telegram", resp[1].Channel)
}

func TestStepsToResponse_Empty(t *testing.T) {
	resp := StepsToResponse([]domain.SequenceStep{})
	assert.Len(t, resp, 0)
	assert.NotNil(t, resp)
}

func TestOutboundMessageToResponse(t *testing.T) {
	now := time.Now().UTC()
	sentAt := now.Add(time.Hour)
	m := &domain.OutboundMessage{
		ID:          uuid.New(),
		ProspectID:  uuid.New(),
		SequenceID:  uuid.New(),
		StepOrder:   1,
		Channel:     domain.StepChannelEmail,
		Body:        "Hello!",
		Status:      domain.OutboundStatusDraft,
		ScheduledAt: now,
		SentAt:      &sentAt,
		CreatedAt:   now,
	}

	resp := OutboundMessageToResponse(m)
	assert.Equal(t, m.ID, resp.ID)
	assert.Equal(t, m.ProspectID, resp.ProspectID)
	assert.Equal(t, m.SequenceID, resp.SequenceID)
	assert.Equal(t, 1, resp.StepOrder)
	assert.Equal(t, "email", resp.Channel)
	assert.Equal(t, "Hello!", resp.Body)
	assert.Equal(t, "draft", resp.Status)
	assert.Equal(t, now, resp.ScheduledAt)
	assert.NotNil(t, resp.SentAt)
	assert.Equal(t, sentAt, *resp.SentAt)
}

func TestOutboundMessageToResponse_NilSentAt(t *testing.T) {
	m := &domain.OutboundMessage{
		ID:     uuid.New(),
		SentAt: nil,
	}

	resp := OutboundMessageToResponse(m)
	assert.Nil(t, resp.SentAt)
}

func TestOutboundMessagesToResponse(t *testing.T) {
	msgs := []domain.OutboundMessage{
		{ID: uuid.New(), Body: "msg1", Channel: domain.StepChannelEmail, Status: domain.OutboundStatusDraft},
		{ID: uuid.New(), Body: "msg2", Channel: domain.StepChannelTelegram, Status: domain.OutboundStatusSent},
	}

	resp := OutboundMessagesToResponse(msgs)
	assert.Len(t, resp, 2)
	assert.Equal(t, "msg1", resp[0].Body)
	assert.Equal(t, "msg2", resp[1].Body)
}

func TestOutboundMessagesToResponse_Empty(t *testing.T) {
	resp := OutboundMessagesToResponse([]domain.OutboundMessage{})
	assert.Len(t, resp, 0)
	assert.NotNil(t, resp)
}

func TestStatsToResponse(t *testing.T) {
	s := &domain.Stats{
		Draft:    10,
		Approved: 5,
		Sent:     20,
		Opened:   8,
		Replied:  3,
		Bounced:  1,
	}

	resp := StatsToResponse(s)
	assert.Equal(t, 10, resp.Draft)
	assert.Equal(t, 5, resp.Approved)
	assert.Equal(t, 20, resp.Sent)
	assert.Equal(t, 8, resp.Opened)
	assert.Equal(t, 3, resp.Replied)
	assert.Equal(t, 1, resp.Bounced)
}
