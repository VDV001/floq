package sequences

import (
	"context"
	"fmt"
	"time"

	"github.com/daniil/floq/internal/sequences/domain"
	"github.com/google/uuid"
)

type UseCase struct {
	repo        domain.Repository
	aiGenerator domain.AIMessageGenerator
	prospects   domain.ProspectReader
	leadCreator domain.LeadCreator
}

func NewUseCase(repo domain.Repository, aiGenerator domain.AIMessageGenerator, prospects domain.ProspectReader, leadCreator domain.LeadCreator) *UseCase {
	return &UseCase{repo: repo, aiGenerator: aiGenerator, prospects: prospects, leadCreator: leadCreator}
}

func (uc *UseCase) ListSequences(ctx context.Context, userID uuid.UUID) ([]domain.Sequence, error) {
	return uc.repo.ListSequences(ctx, userID)
}

func (uc *UseCase) GetSequence(ctx context.Context, id uuid.UUID) (*domain.Sequence, error) {
	return uc.repo.GetSequence(ctx, id)
}

func (uc *UseCase) CreateSequence(ctx context.Context, s *domain.Sequence) error {
	return uc.repo.CreateSequence(ctx, s)
}

func (uc *UseCase) UpdateSequence(ctx context.Context, s *domain.Sequence) error {
	return uc.repo.UpdateSequence(ctx, s)
}

func (uc *UseCase) DeleteSequence(ctx context.Context, id uuid.UUID) error {
	return uc.repo.DeleteSequence(ctx, id)
}

func (uc *UseCase) ToggleActive(ctx context.Context, id uuid.UUID, active bool) error {
	return uc.repo.ToggleActive(ctx, id, active)
}

func (uc *UseCase) ListSteps(ctx context.Context, sequenceID uuid.UUID) ([]domain.SequenceStep, error) {
	return uc.repo.ListSteps(ctx, sequenceID)
}

func (uc *UseCase) CreateStep(ctx context.Context, step *domain.SequenceStep) error {
	return uc.repo.CreateStep(ctx, step)
}

func (uc *UseCase) Launch(ctx context.Context, sequenceID uuid.UUID, prospectIDs []uuid.UUID, sendNow ...bool) error {
	steps, err := uc.repo.ListSteps(ctx, sequenceID)
	if err != nil {
		return fmt.Errorf("launch: list steps: %w", err)
	}
	if len(steps) == 0 {
		return fmt.Errorf("launch: sequence has no steps")
	}

	now := time.Now().UTC()
	immediate := len(sendNow) > 0 && sendNow[0]

	for _, pid := range prospectIDs {
		prospect, err := uc.prospects.GetProspect(ctx, pid)
		if err != nil {
			return fmt.Errorf("launch: get prospect %s: %w", pid, err)
		}
		if prospect == nil {
			return fmt.Errorf("launch: prospect %s not found", pid)
		}

		// Deduplication checks
		if prospect.Status == "converted" || prospect.Status == "opted_out" {
			continue
		}
		if prospect.Status == "in_sequence" {
			continue
		}
		if prospect.VerifyStatus == "invalid" {
			continue
		}
		if prospect.VerifyStatus == "not_checked" && prospect.Email != "" {
			continue
		}

		var cumulativeDelay int
		var previousBody string
		for _, step := range steps {
			cumulativeDelay += step.DelayDays

			var body string
			var genErr error

			switch step.Channel {
			case domain.StepChannelTelegram:
				body, genErr = uc.aiGenerator.GenerateTelegramMessage(ctx, prospect.Name, prospect.Title, prospect.Company, prospect.Context, step.PromptHint, previousBody, prospect.Source)
			case domain.StepChannelPhoneCall:
				body, genErr = uc.aiGenerator.GenerateCallBrief(ctx, prospect.Name, prospect.Title, prospect.Company, prospect.Context, step.PromptHint, previousBody)
			default: // "email" or empty
				body, genErr = uc.aiGenerator.GenerateColdMessage(ctx, prospect.Name, prospect.Title, prospect.Company, prospect.Context, step.PromptHint, previousBody, prospect.Source)
			}
			if genErr != nil {
				return fmt.Errorf("launch: generate message for prospect %s step %d: %w", pid, step.StepOrder, genErr)
			}

			msg := &domain.OutboundMessage{
				ID:          uuid.New(),
				ProspectID:  pid,
				SequenceID:  sequenceID,
				StepOrder:   step.StepOrder,
				Channel:     step.Channel,
				Body:        body,
				Status:      domain.OutboundStatusDraft,
				ScheduledAt: func() time.Time {
					if immediate {
						return now
					}
					return now.AddDate(0, 0, cumulativeDelay)
				}(),
				CreatedAt:   now,
			}
			if err := uc.repo.CreateOutboundMessage(ctx, msg); err != nil {
				return fmt.Errorf("launch: create outbound message: %w", err)
			}

			previousBody = body
		}

		if err := uc.prospects.UpdateStatus(ctx, pid, "in_sequence"); err != nil {
			return fmt.Errorf("launch: update prospect status: %w", err)
		}
	}

	return nil
}

func (uc *UseCase) ApproveMessage(ctx context.Context, id uuid.UUID) error {
	return uc.repo.UpdateOutboundStatus(ctx, id, domain.OutboundStatusApproved)
}

func (uc *UseCase) RejectMessage(ctx context.Context, id uuid.UUID) error {
	return uc.repo.UpdateOutboundStatus(ctx, id, domain.OutboundStatusRejected)
}

func (uc *UseCase) EditMessage(ctx context.Context, id uuid.UUID, body string) error {
	return uc.repo.UpdateOutboundBody(ctx, id, body)
}

func (uc *UseCase) DeleteStep(ctx context.Context, stepID uuid.UUID) error {
	return uc.repo.DeleteStep(ctx, stepID)
}

func (uc *UseCase) GetQueue(ctx context.Context, userID uuid.UUID) ([]domain.OutboundMessage, error) {
	return uc.repo.ListOutboundQueue(ctx, userID)
}

func (uc *UseCase) GetSent(ctx context.Context, userID uuid.UUID) ([]domain.OutboundMessage, error) {
	return uc.repo.ListSentMessages(ctx, userID)
}

func (uc *UseCase) GetStats(ctx context.Context, userID uuid.UUID) (*domain.Stats, error) {
	return uc.repo.GetStats(ctx, userID)
}

func (uc *UseCase) ConvertToLead(ctx context.Context, prospectID uuid.UUID) error {
	prospect, err := uc.prospects.GetProspect(ctx, prospectID)
	if err != nil {
		return fmt.Errorf("convert: get prospect: %w", err)
	}
	if prospect == nil {
		return fmt.Errorf("convert: prospect not found")
	}

	leadID, err := uc.leadCreator.CreateLeadFromProspect(ctx, prospect, prospect.UserID)
	if err != nil {
		return fmt.Errorf("convert: create lead: %w", err)
	}
	_ = leadID

	if err := uc.prospects.UpdateStatus(ctx, prospectID, "converted"); err != nil {
		return fmt.Errorf("convert: update prospect: %w", err)
	}

	return nil
}
