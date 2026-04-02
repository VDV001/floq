package sequences

import (
	"context"
	"fmt"
	"time"

	"github.com/daniil/floq/internal/ai"
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
}

func NewUseCase(repo *Repository, aiClient *ai.AIClient, prospectRepo *prospects.Repository) *UseCase {
	return &UseCase{repo: repo, aiClient: aiClient, prospectRepo: prospectRepo}
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

		var cumulativeDelay int
		var previousBody string
		for _, step := range steps {
			cumulativeDelay += step.DelayDays

			body, err := uc.aiClient.GenerateColdMessage(ctx, prospect.Name, prospect.Title, prospect.Company, step.PromptHint, previousBody)
			if err != nil {
				return fmt.Errorf("launch: generate message for prospect %s step %d: %w", pid, step.StepOrder, err)
			}

			msg := &OutboundMessage{
				ID:          uuid.New(),
				ProspectID:  pid,
				SequenceID:  sequenceID,
				StepOrder:   step.StepOrder,
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
