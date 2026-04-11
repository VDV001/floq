package leads

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
)

type UseCase struct {
	repo   domain.Repository
	ai     domain.AIService
	sender domain.MessageSender
}

func NewUseCase(repo domain.Repository, ai domain.AIService, sender domain.MessageSender) *UseCase {
	return &UseCase{repo: repo, ai: ai, sender: sender}
}

// SetSender sets the message sender after construction (e.g. when the Telegram bot
// is initialised later than the use case).
func (uc *UseCase) SetSender(sender domain.MessageSender) {
	uc.sender = sender
}

func (uc *UseCase) ListLeads(ctx context.Context, userID uuid.UUID) ([]domain.Lead, error) {
	return uc.repo.ListLeads(ctx, userID)
}

func (uc *UseCase) GetLead(ctx context.Context, id uuid.UUID) (*domain.Lead, error) {
	return uc.repo.GetLead(ctx, id)
}

func (uc *UseCase) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	target := domain.LeadStatus(status)
	if !target.IsValid() {
		return fmt.Errorf("invalid status: %q", status)
	}

	lead, err := uc.repo.GetLead(ctx, id)
	if err != nil {
		return fmt.Errorf("get lead: %w", err)
	}
	if lead == nil {
		return fmt.Errorf("lead not found")
	}

	if err := lead.TransitionTo(target); err != nil {
		return err
	}

	return uc.repo.UpdateLeadStatus(ctx, id, target)
}

func (uc *UseCase) GetMessages(ctx context.Context, leadID uuid.UUID) ([]domain.Message, error) {
	return uc.repo.ListMessages(ctx, leadID)
}

func (uc *UseCase) SendMessage(ctx context.Context, leadID uuid.UUID, body string) (*domain.Message, error) {
	lead, err := uc.repo.GetLead(ctx, leadID)
	if err != nil {
		return nil, fmt.Errorf("get lead: %w", err)
	}

	// Send via the message sender if available and applicable
	if lead.Channel == domain.ChannelTelegram && lead.TelegramChatID != nil && uc.sender != nil {
		if err := uc.sender.SendMessage(ctx, lead, body); err != nil {
			return nil, fmt.Errorf("send message: %w", err)
		}
	}

	// Save to DB
	msg := domain.NewMessage(leadID, domain.DirectionOutbound, body)
	if err := uc.repo.CreateMessage(ctx, msg); err != nil {
		return nil, err
	}

	// Auto-transition: qualified → in_conversation on first outbound message
	if lead.Status == domain.StatusQualified {
		_ = uc.repo.UpdateLeadStatus(ctx, leadID, domain.StatusInConversation)
	}

	return msg, nil
}

func (uc *UseCase) GetQualification(ctx context.Context, leadID uuid.UUID) (*domain.Qualification, error) {
	return uc.repo.GetQualification(ctx, leadID)
}

func (uc *UseCase) QualifyLead(ctx context.Context, leadID uuid.UUID) (*domain.Qualification, error) {
	lead, err := uc.repo.GetLead(ctx, leadID)
	if err != nil {
		return nil, err
	}
	if lead == nil {
		return nil, fmt.Errorf("lead not found")
	}

	q, err := uc.ai.Qualify(ctx, lead.ContactName, lead.Channel, lead.FirstMessage)
	if err != nil {
		return nil, err
	}

	q.ID = uuid.New()
	q.LeadID = lead.ID
	q.GeneratedAt = time.Now().UTC()

	if err := uc.repo.UpsertQualification(ctx, q); err != nil {
		return nil, err
	}

	if err := uc.repo.UpdateLeadStatus(ctx, leadID, domain.StatusQualified); err != nil {
		return nil, err
	}

	return q, nil
}

func (uc *UseCase) GetDraft(ctx context.Context, leadID uuid.UUID) (*domain.Draft, error) {
	return uc.repo.GetLatestDraft(ctx, leadID)
}

func (uc *UseCase) RegenerateDraft(ctx context.Context, leadID uuid.UUID) (*domain.Draft, error) {
	lead, err := uc.repo.GetLead(ctx, leadID)
	if err != nil {
		return nil, err
	}
	if lead == nil {
		return nil, fmt.Errorf("lead not found")
	}

	qual, err := uc.repo.GetQualification(ctx, leadID)
	if err != nil {
		return nil, err
	}

	// Build context-enriched first message for the AI
	firstMsg := lead.FirstMessage
	if qual != nil {
		if b, err := json.Marshal(qual); err == nil {
			firstMsg = firstMsg + "\n\nQualification: " + string(b)
		}
	}

	body, err := uc.ai.DraftReply(ctx, lead.ContactName, firstMsg)
	if err != nil {
		return nil, err
	}

	d := &domain.Draft{
		ID:        uuid.New(),
		LeadID:    lead.ID,
		Body:      body,
		CreatedAt: time.Now().UTC(),
	}

	if err := uc.repo.CreateDraft(ctx, d); err != nil {
		return nil, err
	}

	return d, nil
}
