package leads

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/daniil/floq/internal/ai"
	"github.com/google/uuid"
)

type UseCase struct {
	repo *Repository
	ai   *ai.AIClient
}

func NewUseCase(repo *Repository, aiClient *ai.AIClient) *UseCase {
	return &UseCase{repo: repo, ai: aiClient}
}

func (uc *UseCase) ListLeads(ctx context.Context, userID uuid.UUID) ([]Lead, error) {
	return uc.repo.ListLeads(ctx, userID)
}

func (uc *UseCase) GetLead(ctx context.Context, id uuid.UUID) (*Lead, error) {
	return uc.repo.GetLead(ctx, id)
}

func (uc *UseCase) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return uc.repo.UpdateLeadStatus(ctx, id, status)
}

func (uc *UseCase) GetMessages(ctx context.Context, leadID uuid.UUID) ([]Message, error) {
	return uc.repo.ListMessages(ctx, leadID)
}

func (uc *UseCase) SendMessage(ctx context.Context, leadID uuid.UUID, body string) (*Message, error) {
	msg := &Message{
		ID:        uuid.New(),
		LeadID:    leadID,
		Direction: "outbound",
		Body:      body,
		SentAt:    time.Now().UTC(),
	}
	if err := uc.repo.CreateMessage(ctx, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func (uc *UseCase) GetQualification(ctx context.Context, leadID uuid.UUID) (*Qualification, error) {
	return uc.repo.GetQualification(ctx, leadID)
}

func (uc *UseCase) QualifyLead(ctx context.Context, leadID uuid.UUID) (*Qualification, error) {
	lead, err := uc.repo.GetLead(ctx, leadID)
	if err != nil {
		return nil, err
	}
	if lead == nil {
		return nil, fmt.Errorf("lead not found")
	}

	result, err := uc.ai.Qualify(ctx, lead.ContactName, lead.Channel, lead.FirstMessage)
	if err != nil {
		return nil, err
	}

	q := &Qualification{
		ID:                uuid.New(),
		LeadID:            lead.ID,
		IdentifiedNeed:    result.IdentifiedNeed,
		EstimatedBudget:   result.EstimatedBudget,
		Deadline:          result.Deadline,
		Score:             result.Score,
		ScoreReason:       result.ScoreReason,
		RecommendedAction: result.RecommendedAction,
		ProviderUsed:      uc.ai.ProviderName(),
		GeneratedAt:       time.Now().UTC(),
	}

	if err := uc.repo.UpsertQualification(ctx, q); err != nil {
		return nil, err
	}

	if err := uc.repo.UpdateLeadStatus(ctx, leadID, "qualified"); err != nil {
		return nil, err
	}

	return q, nil
}

func (uc *UseCase) GetDraft(ctx context.Context, leadID uuid.UUID) (*Draft, error) {
	return uc.repo.GetLatestDraft(ctx, leadID)
}

func (uc *UseCase) RegenerateDraft(ctx context.Context, leadID uuid.UUID) (*Draft, error) {
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

	qualJSON := "{}"
	if qual != nil {
		if b, err := json.Marshal(qual); err == nil {
			qualJSON = string(b)
		}
	}

	body, err := uc.ai.DraftReply(ctx, lead.ContactName, lead.Company, lead.Channel, lead.FirstMessage, qualJSON)
	if err != nil {
		return nil, err
	}

	d := &Draft{
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
