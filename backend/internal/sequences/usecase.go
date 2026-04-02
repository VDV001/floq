package sequences

import (
	"context"
	"fmt"
	"time"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/leads"
	"github.com/daniil/floq/internal/prospects"
	"github.com/google/uuid"
)

type Stats struct {
	Draft    int `json:"draft"`
	Approved int `json:"approved"`
	Sent     int `json:"sent"`
}

type UseCase struct {
	repo         *Repository
	aiClient     *ai.AIClient
	prospectRepo *prospects.Repository
	leadsRepo    *leads.Repository
}

func NewUseCase(repo *Repository, aiClient *ai.AIClient, prospectRepo *prospects.Repository, leadsRepo *leads.Repository) *UseCase {
	return &UseCase{repo: repo, aiClient: aiClient, prospectRepo: prospectRepo, leadsRepo: leadsRepo}
}

func (uc *UseCase) ListSequences(ctx context.Context, userID uuid.UUID) ([]Sequence, error) {
	return uc.repo.ListSequences(ctx, userID)
}

func (uc *UseCase) GetSequence(ctx context.Context, id uuid.UUID) (*Sequence, error) {
	return uc.repo.GetSequence(ctx, id)
}

func (uc *UseCase) CreateSequence(ctx context.Context, s *Sequence) error {
	return uc.repo.CreateSequence(ctx, s)
}

func (uc *UseCase) UpdateSequence(ctx context.Context, s *Sequence) error {
	return uc.repo.UpdateSequence(ctx, s)
}

func (uc *UseCase) DeleteSequence(ctx context.Context, id uuid.UUID) error {
	return uc.repo.DeleteSequence(ctx, id)
}

func (uc *UseCase) ToggleActive(ctx context.Context, id uuid.UUID, active bool) error {
	return uc.repo.ToggleActive(ctx, id, active)
}

func (uc *UseCase) ListSteps(ctx context.Context, sequenceID uuid.UUID) ([]SequenceStep, error) {
	return uc.repo.ListSteps(ctx, sequenceID)
}

func (uc *UseCase) CreateStep(ctx context.Context, step *SequenceStep) error {
	return uc.repo.CreateStep(ctx, step)
}

func (uc *UseCase) Launch(ctx context.Context, sequenceID uuid.UUID, prospectIDs []uuid.UUID) error {
	steps, err := uc.repo.ListSteps(ctx, sequenceID)
	if err != nil {
		return fmt.Errorf("launch: list steps: %w", err)
	}
	if len(steps) == 0 {
		return fmt.Errorf("launch: sequence has no steps")
	}

	now := time.Now().UTC()

	for _, pid := range prospectIDs {
		prospect, err := uc.prospectRepo.GetProspect(ctx, pid)
		if err != nil {
			return fmt.Errorf("launch: get prospect %s: %w", pid, err)
		}
		if prospect == nil {
			return fmt.Errorf("launch: prospect %s not found", pid)
		}

		// Deduplication checks
		if prospect.Status == "converted" || prospect.Status == "opted_out" {
			continue // skip — already in inbox or opted out
		}
		if prospect.Status == "in_sequence" {
			continue // skip — already in a sequence
		}
		if prospect.VerifyStatus == "invalid" {
			continue // skip — verified as invalid email
		}

		var cumulativeDelay int
		var previousBody string
		for _, step := range steps {
			cumulativeDelay += step.DelayDays

			var body string
			var genErr error

			switch step.Channel {
			case "telegram":
				body, genErr = uc.aiClient.GenerateTelegramMessage(ctx, prospect.Name, prospect.Title, prospect.Company, prospect.Context, step.PromptHint, previousBody)
			case "phone_call":
				body, genErr = uc.aiClient.GenerateCallBrief(ctx, prospect.Name, prospect.Title, prospect.Company, prospect.Context, step.PromptHint, previousBody)
			default: // "email" or empty
				body, genErr = uc.aiClient.GenerateColdMessage(ctx, prospect.Name, prospect.Title, prospect.Company, prospect.Context, step.PromptHint, previousBody)
			}
			if genErr != nil {
				return fmt.Errorf("launch: generate message for prospect %s step %d: %w", pid, step.StepOrder, genErr)
			}

			msg := &OutboundMessage{
				ID:          uuid.New(),
				ProspectID:  pid,
				SequenceID:  sequenceID,
				StepOrder:   step.StepOrder,
				Channel:     step.Channel,
				Body:        body,
				Status:      "draft",
				ScheduledAt: now.AddDate(0, 0, cumulativeDelay),
				CreatedAt:   now,
			}
			if err := uc.repo.CreateOutboundMessage(ctx, msg); err != nil {
				return fmt.Errorf("launch: create outbound message: %w", err)
			}

			previousBody = body
		}

		if err := uc.prospectRepo.UpdateStatus(ctx, pid, "in_sequence"); err != nil {
			return fmt.Errorf("launch: update prospect status: %w", err)
		}
	}

	return nil
}

func (uc *UseCase) ApproveMessage(ctx context.Context, id uuid.UUID) error {
	return uc.repo.UpdateOutboundStatus(ctx, id, "approved")
}

func (uc *UseCase) RejectMessage(ctx context.Context, id uuid.UUID) error {
	return uc.repo.UpdateOutboundStatus(ctx, id, "rejected")
}

func (uc *UseCase) EditMessage(ctx context.Context, id uuid.UUID, body string) error {
	return uc.repo.UpdateOutboundBody(ctx, id, body)
}

func (uc *UseCase) GetQueue(ctx context.Context, userID uuid.UUID) ([]OutboundMessage, error) {
	return uc.repo.ListOutboundQueue(ctx, userID)
}

func (uc *UseCase) GetStats(ctx context.Context, userID uuid.UUID) (*Stats, error) {
	return uc.repo.GetStats(ctx, userID)
}

func (uc *UseCase) ConvertToLead(ctx context.Context, prospectID uuid.UUID) error {
	prospect, err := uc.prospectRepo.GetProspect(ctx, prospectID)
	if err != nil {
		return fmt.Errorf("convert: get prospect: %w", err)
	}
	if prospect == nil {
		return fmt.Errorf("convert: prospect not found")
	}

	// Create a lead from the prospect data
	leadID := uuid.New()
	now := time.Now().UTC()
	lead := &leads.Lead{
		ID:           leadID,
		UserID:       prospect.UserID,
		Channel:      "email",
		ContactName:  prospect.Name,
		Company:      prospect.Company,
		FirstMessage: "Ответ на outbound секвенцию",
		Status:       "new",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := uc.leadsRepo.CreateLead(ctx, lead); err != nil {
		return fmt.Errorf("convert: create lead: %w", err)
	}

	// Update prospect as converted
	if err := uc.prospectRepo.ConvertToLead(ctx, prospectID, leadID); err != nil {
		return fmt.Errorf("convert: update prospect: %w", err)
	}

	return nil
}
