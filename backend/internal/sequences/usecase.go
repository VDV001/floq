package sequences

import (
	"context"
	"fmt"
	"strings"
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

	// Build feedback examples string once for the entire launch (use first prospect's userID).
	var feedbackExamples string
	if len(prospectIDs) > 0 {
		firstProspect, _ := uc.prospects.GetProspect(ctx, prospectIDs[0])
		if firstProspect != nil {
			feedbackExamples = uc.buildFeedbackExamples(ctx, firstProspect.UserID)
		}
	}

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

		// Load conversation history for this prospect (previously sent/approved messages).
		var conversationContext string
		history, histErr := uc.repo.GetConversationHistory(ctx, pid)
		if histErr == nil && len(history) > 0 {
			var b strings.Builder
			b.WriteString("Наши сообщения:")
			for i, entry := range history {
				fmt.Fprintf(&b, "\n%d. \"%s\"", i+1, entry.Body)
			}
			conversationContext = b.String()
		}

		var cumulativeDelay int
		var previousBody string
		for _, step := range steps {
			cumulativeDelay += step.DelayDays

			// Use conversation history as context if available, otherwise fall back to previous step body.
			prevCtx := previousBody
			if conversationContext != "" {
				prevCtx = conversationContext
			}

			var body string
			var genErr error

			switch step.Channel {
			case domain.StepChannelTelegram:
				body, genErr = uc.aiGenerator.GenerateTelegramMessage(ctx, prospect.Name, prospect.Title, prospect.Company, prospect.Context, step.PromptHint, prevCtx, prospect.Source, feedbackExamples)
			case domain.StepChannelPhoneCall:
				body, genErr = uc.aiGenerator.GenerateCallBrief(ctx, prospect.Name, prospect.Title, prospect.Company, prospect.Context, step.PromptHint, prevCtx)
			default: // "email" or empty
				body, genErr = uc.aiGenerator.GenerateColdMessage(ctx, prospect.Name, prospect.Title, prospect.Company, prospect.Context, step.PromptHint, prevCtx, prospect.Source, feedbackExamples)
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
	// Read original message before updating.
	msg, err := uc.repo.GetOutboundMessage(ctx, id)
	if err != nil {
		return fmt.Errorf("edit message: get original: %w", err)
	}
	if msg == nil {
		return fmt.Errorf("edit message: message not found")
	}

	originalBody := msg.Body

	// Update the body.
	if err := uc.repo.UpdateOutboundBody(ctx, id, body); err != nil {
		return err
	}

	// Save feedback if the body actually changed.
	if originalBody != body {
		// Get prospect context for the feedback record.
		var prospectContext string
		prospect, pErr := uc.prospects.GetProspect(ctx, msg.ProspectID)
		if pErr == nil && prospect != nil {
			prospectContext = prospect.Context
		}

		channel := string(msg.Channel)
		if channel == "" {
			channel = "email"
		}

		_ = uc.repo.SavePromptFeedback(ctx, func() uuid.UUID {
			if prospect != nil {
				return prospect.UserID
			}
			return uuid.Nil
		}(), originalBody, body, prospectContext, channel)
	}

	return nil
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

func (uc *UseCase) buildFeedbackExamples(ctx context.Context, userID uuid.UUID) string {
	feedback, err := uc.repo.GetRecentFeedback(ctx, userID, 3)
	if err != nil || len(feedback) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("Примеры правок менеджера (учитывай стиль):")
	for _, f := range feedback {
		b.WriteString("\nБыло: \"")
		b.WriteString(f.OriginalBody)
		b.WriteString("\" → Стало: \"")
		b.WriteString(f.EditedBody)
		b.WriteString("\"")
	}
	return b.String()
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
