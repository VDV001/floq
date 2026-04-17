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
	tx          domain.TxManager
}

func NewUseCase(repo domain.Repository, aiGenerator domain.AIMessageGenerator, prospects domain.ProspectReader, leadCreator domain.LeadCreator, opts ...UseCaseOption) *UseCase {
	uc := &UseCase{repo: repo, aiGenerator: aiGenerator, prospects: prospects, leadCreator: leadCreator}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

// UseCaseOption configures the UseCase.
type UseCaseOption func(*UseCase)

// WithTxManager sets the transaction manager.
func WithTxManager(tx domain.TxManager) UseCaseOption {
	return func(uc *UseCase) { uc.tx = tx }
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

func (uc *UseCase) UpdateSequence(ctx context.Context, id uuid.UUID, name string) error {
	s, err := uc.repo.GetSequence(ctx, id)
	if err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("sequence not found")
	}
	// Route through the domain method so the non-empty-name invariant is
	// enforced here, not just at creation time.
	if err := s.Rename(name); err != nil {
		return fmt.Errorf("update sequence: %w", err)
	}
	return uc.repo.UpdateSequence(ctx, s)
}

func (uc *UseCase) DeleteSequence(ctx context.Context, id uuid.UUID) error {
	return uc.repo.DeleteSequence(ctx, id)
}

// ToggleActive loads the sequence, applies the domain intent method
// (Activate/Deactivate), and persists. This ensures future rules on
// activation (e.g. "can't activate a sequence with no steps") live on the
// entity, not scattered across handlers/usecases.
func (uc *UseCase) ToggleActive(ctx context.Context, id uuid.UUID, active bool) error {
	s, err := uc.repo.GetSequence(ctx, id)
	if err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("sequence not found")
	}
	if active {
		s.Activate()
	} else {
		s.Deactivate()
	}
	return uc.repo.UpdateSequence(ctx, s)
}

func (uc *UseCase) ListSteps(ctx context.Context, sequenceID uuid.UUID) ([]domain.SequenceStep, error) {
	return uc.repo.ListSteps(ctx, sequenceID)
}

func (uc *UseCase) CreateStep(ctx context.Context, step *domain.SequenceStep) error {
	return uc.repo.CreateStep(ctx, step)
}

func (uc *UseCase) Launch(ctx context.Context, sequenceID uuid.UUID, prospectIDs []uuid.UUID, sendNow ...bool) error {
	if uc.tx != nil {
		return uc.tx.WithTx(ctx, func(txCtx context.Context) error {
			return uc.launchInner(txCtx, sequenceID, prospectIDs, sendNow...)
		})
	}
	return uc.launchInner(ctx, sequenceID, prospectIDs, sendNow...)
}

func (uc *UseCase) launchInner(ctx context.Context, sequenceID uuid.UUID, prospectIDs []uuid.UUID, sendNow ...bool) error {
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

		if !prospect.IsEligibleForSequence {
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

			scheduledAt := now.AddDate(0, 0, cumulativeDelay)
			if immediate {
				scheduledAt = now
			}
			msg := domain.NewOutboundMessage(pid, sequenceID, step.StepOrder, step.Channel, body, scheduledAt)
			if err := uc.repo.CreateOutboundMessage(ctx, msg); err != nil {
				return fmt.Errorf("launch: create outbound message: %w", err)
			}

			previousBody = body
		}

		if err := uc.prospects.MarkInSequence(ctx, pid); err != nil {
			return fmt.Errorf("launch: update prospect status: %w", err)
		}
	}

	return nil
}

// ApproveMessage loads the message, asks the domain entity to transition to
// Approved (fails if the current status forbids it), then persists the result.
// The state machine (see domain.OutboundMessage.TransitionTo) is the single
// source of truth for legal transitions — the repo is dumb persistence.
func (uc *UseCase) ApproveMessage(ctx context.Context, id uuid.UUID) error {
	msg, err := uc.repo.GetOutboundMessage(ctx, id)
	if err != nil {
		return fmt.Errorf("approve message: load: %w", err)
	}
	if msg == nil {
		return fmt.Errorf("approve message: not found")
	}
	if err := msg.TransitionTo(domain.OutboundStatusApproved); err != nil {
		return fmt.Errorf("approve message: %w", err)
	}
	return uc.repo.UpdateOutboundStatus(ctx, id, msg.Status)
}

// RejectMessage follows the same load → domain.TransitionTo → persist pattern
// as ApproveMessage. Draft and Approved are legal predecessors; anything else
// (Sent, Bounced, already Rejected) returns an error from the domain.
func (uc *UseCase) RejectMessage(ctx context.Context, id uuid.UUID) error {
	msg, err := uc.repo.GetOutboundMessage(ctx, id)
	if err != nil {
		return fmt.Errorf("reject message: load: %w", err)
	}
	if msg == nil {
		return fmt.Errorf("reject message: not found")
	}
	if err := msg.TransitionTo(domain.OutboundStatusRejected); err != nil {
		return fmt.Errorf("reject message: %w", err)
	}
	return uc.repo.UpdateOutboundStatus(ctx, id, msg.Status)
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
			channel = string(domain.StepChannelEmail)
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

func (uc *UseCase) MarkOpened(ctx context.Context, id uuid.UUID) error {
	return uc.repo.MarkOpened(ctx, id)
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

// ConvertToLead creates a lead from a prospect and marks the prospect as
// converted. Wrapped in a transaction via TxManager — if lead creation
// succeeds but the status update fails, the whole thing rolls back so we
// never leave a dangling "prospect still in_sequence but lead exists"
// state. If no TxManager is configured (legacy wiring / tests), falls back
// to best-effort sequential execution.
func (uc *UseCase) ConvertToLead(ctx context.Context, prospectID uuid.UUID) error {
	convert := func(txCtx context.Context) error {
		prospect, err := uc.prospects.GetProspect(txCtx, prospectID)
		if err != nil {
			return fmt.Errorf("convert: get prospect: %w", err)
		}
		if prospect == nil {
			return fmt.Errorf("convert: prospect not found")
		}
		if _, err := uc.leadCreator.CreateLeadFromProspect(txCtx, prospect, prospect.UserID); err != nil {
			return fmt.Errorf("convert: create lead: %w", err)
		}
		if err := uc.prospects.MarkConverted(txCtx, prospectID); err != nil {
			return fmt.Errorf("convert: update prospect: %w", err)
		}
		return nil
	}
	if uc.tx == nil {
		return convert(ctx)
	}
	return uc.tx.WithTx(ctx, convert)
}
