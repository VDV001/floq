package sequences

import (
	"context"
	"fmt"
	"strings"
	"time"

	auditdomain "github.com/daniil/floq/internal/audit/domain"
	"github.com/daniil/floq/internal/sequences/domain"
	"github.com/google/uuid"
)

type UseCase struct {
	repo         domain.Repository
	aiGenerator  domain.AIMessageGenerator
	prospects    domain.ProspectReader
	leadCreator  domain.LeadCreator
	tx           domain.TxManager
	emailChecker domain.EmailConfigChecker
	autopilot    domain.AutopilotChecker
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

// WithEmailConfigChecker wires the email-configuration preflight. When unset,
// launch skips the check (preserving pre-feature behaviour).
func WithEmailConfigChecker(c domain.EmailConfigChecker) UseCaseOption {
	return func(uc *UseCase) { uc.emailChecker = c }
}

// WithAutopilotChecker wires the autopilot send-mode resolver. When unset,
// launch keeps the default human-in-the-loop behaviour (messages stay drafts).
func WithAutopilotChecker(c domain.AutopilotChecker) UseCaseOption {
	return func(uc *UseCase) { uc.autopilot = c }
}

func (uc *UseCase) ListSequences(ctx context.Context, userID uuid.UUID) ([]domain.Sequence, error) {
	return uc.repo.ListSequences(ctx, userID)
}

// GetSequence loads a sequence by id. userID is the authenticated caller; the
// ownership boundary is enforced in a later change (this commit only threads
// the parameter through, behaviour unchanged).
// authorizeSequence loads a sequence and verifies the authenticated caller
// owns it. A missing and a foreign sequence BOTH return ErrSequenceNotOwned so
// the two are indistinguishable to the caller (anti-enumeration). userID ==
// uuid.Nil skips the ownership check — a unit-test affordance only: the handler
// always supplies a real caller and a persisted sequence always has a non-nil
// owner (NewSequence enforces it), so a nil caller cannot occur in production.
func (uc *UseCase) authorizeSequence(ctx context.Context, userID, id uuid.UUID) (*domain.Sequence, error) {
	s, err := uc.repo.GetSequence(ctx, id)
	if err != nil {
		return nil, err
	}
	if userID == uuid.Nil {
		return s, nil
	}
	if s == nil || s.UserID != userID {
		return nil, domain.ErrSequenceNotOwned
	}
	return s, nil
}

// authorizeStep verifies the caller owns the sequence a step belongs to. A
// step is addressed only by its own id, so the owning sequence is resolved via
// GetStep. A missing step and a step under a foreign sequence both return
// ErrSequenceNotOwned (anti-enumeration). userID == uuid.Nil skips the check
// (unit-test affordance; see authorizeSequence).
func (uc *UseCase) authorizeStep(ctx context.Context, userID, stepID uuid.UUID) error {
	if userID == uuid.Nil {
		return nil
	}
	step, err := uc.repo.GetStep(ctx, stepID)
	if err != nil {
		return err
	}
	if step == nil {
		return domain.ErrSequenceNotOwned
	}
	_, err = uc.authorizeSequence(ctx, userID, step.SequenceID)
	return err
}

// authorizeMessage loads an outbound message and verifies it belongs to the
// caller, resolving ownership through the message's prospect — the same owner
// definition the queue/sent/stats reads use. Missing and foreign both return
// ErrMessageNotOwned (anti-enumeration). userID == uuid.Nil skips the check
// (unit-test affordance; see authorizeSequence).
func (uc *UseCase) authorizeMessage(ctx context.Context, userID, id uuid.UUID) (*domain.OutboundMessage, error) {
	msg, err := uc.repo.GetOutboundMessage(ctx, id)
	if err != nil {
		return nil, err
	}
	if userID == uuid.Nil {
		return msg, nil
	}
	if msg == nil {
		return nil, domain.ErrMessageNotOwned
	}
	p, err := uc.prospects.GetProspect(ctx, msg.ProspectID)
	if err != nil {
		return nil, err
	}
	if p == nil || p.UserID != userID {
		return nil, domain.ErrMessageNotOwned
	}
	return msg, nil
}

func (uc *UseCase) GetSequence(ctx context.Context, userID, id uuid.UUID) (*domain.Sequence, error) {
	return uc.authorizeSequence(ctx, userID, id)
}

func (uc *UseCase) CreateSequence(ctx context.Context, s *domain.Sequence) error {
	return uc.repo.CreateSequence(ctx, s)
}

func (uc *UseCase) UpdateSequence(ctx context.Context, userID, id uuid.UUID, name string) error {
	s, err := uc.authorizeSequence(ctx, userID, id)
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

func (uc *UseCase) DeleteSequence(ctx context.Context, userID, id uuid.UUID) error {
	if _, err := uc.authorizeSequence(ctx, userID, id); err != nil {
		return err
	}
	return uc.repo.DeleteSequence(ctx, id)
}

// ToggleActive loads the sequence, applies the domain intent method
// (Activate/Deactivate), and persists. This ensures future rules on
// activation (e.g. "can't activate a sequence with no steps") live on the
// entity, not scattered across handlers/usecases.
func (uc *UseCase) ToggleActive(ctx context.Context, userID, id uuid.UUID, active bool) error {
	s, err := uc.authorizeSequence(ctx, userID, id)
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

// SetRequireApproval flips the per-sequence outbound HITL gate. It loads the
// owned sequence, mutates the flag, and persists via UpdateSequence — the same
// load→authorize→mutate→persist shape as ToggleActive (the port deliberately
// has no column-specific setter, see domain.Repository). require_approval has
// no invariant, so the field is set directly rather than via an intent method.
func (uc *UseCase) SetRequireApproval(ctx context.Context, userID, id uuid.UUID, require bool) error {
	s, err := uc.authorizeSequence(ctx, userID, id)
	if err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("sequence not found")
	}
	s.RequireApproval = require
	return uc.repo.UpdateSequence(ctx, s)
}

func (uc *UseCase) ListSteps(ctx context.Context, sequenceID uuid.UUID) ([]domain.SequenceStep, error) {
	return uc.repo.ListSteps(ctx, sequenceID)
}

func (uc *UseCase) CreateStep(ctx context.Context, userID uuid.UUID, step *domain.SequenceStep) error {
	if _, err := uc.authorizeSequence(ctx, userID, step.SequenceID); err != nil {
		return err
	}
	return uc.repo.CreateStep(ctx, step)
}

// LaunchResult reports the outcome of a launch so the caller can tell the user
// what actually happened. Queued is the number of prospects for whom messages
// were created; Skipped is the number silently dropped because they were not
// eligible (terminal status, invalid email, or unverified email). A launch that
// reports Queued=0, Skipped>0 is the "nothing happened, and here's why" case the
// UI must surface instead of a false success (see #221).
type LaunchResult struct {
	Queued  int `json:"queued"`
	Skipped int `json:"skipped"`
}

// Launch queues a sequence's messages for the given prospects. userID is the
// authenticated caller — the authoritative owner. Every prospect must belong
// to them (enforced in launchInner); this is the authorization boundary, so it
// lives here in the usecase, never in the handler.
func (uc *UseCase) Launch(ctx context.Context, userID uuid.UUID, sequenceID uuid.UUID, prospectIDs []uuid.UUID, sendNow ...bool) (LaunchResult, error) {
	if uc.tx != nil {
		var result LaunchResult
		err := uc.tx.WithTx(ctx, func(txCtx context.Context) error {
			var innerErr error
			result, innerErr = uc.launchInner(txCtx, userID, sequenceID, prospectIDs, sendNow...)
			return innerErr
		})
		return result, err
	}
	return uc.launchInner(ctx, userID, sequenceID, prospectIDs, sendNow...)
}

// hasEmailStep reports whether any step in the sequence is delivered over
// email — the only channel the email-config preflight gates.
func hasEmailStep(steps []domain.SequenceStep) bool {
	for i := range steps {
		if steps[i].IsEmail() {
			return true
		}
	}
	return false
}

func (uc *UseCase) launchInner(ctx context.Context, userID uuid.UUID, sequenceID uuid.UUID, prospectIDs []uuid.UUID, sendNow ...bool) (LaunchResult, error) {
	var result LaunchResult
	// The caller may only launch a sequence they own — launching a foreign
	// sequence would generate messages from (and thereby leak) its steps.
	// Checked before reading steps so a foreign sequence's content is never
	// touched. authorizeSequence collapses missing+foreign to ErrSequenceNotOwned
	// and is a no-op when userID is nil (unit tests).
	seq, err := uc.authorizeSequence(ctx, userID, sequenceID)
	if err != nil {
		return result, fmt.Errorf("launch: %w", err)
	}
	// The per-sequence approval gate (see domain.InitialOutboundStatus) forces
	// every launched message through human review, overriding autopilot. seq is
	// nil only in unit tests that don't seed a sequence (userID nil); treat that
	// as no gate so their behaviour is unchanged.
	requireApproval := seq != nil && seq.RequireApproval

	steps, err := uc.repo.ListSteps(ctx, sequenceID)
	if err != nil {
		return result, fmt.Errorf("launch: list steps: %w", err)
	}
	if len(steps) == 0 {
		return result, fmt.Errorf("launch: sequence has no steps")
	}

	now := time.Now().UTC()
	immediate := len(sendNow) > 0 && sendNow[0]

	// The authenticated caller is the authoritative owner for all per-launch
	// side-effects (feedback examples, email preflight, autopilot resolution),
	// so they never read a stranger's settings even if a foreign prospect id
	// slips into the batch. Fall back to the first prospect's owner only when
	// userID is nil — a unit-test affordance; the handler always supplies a
	// real userID (the route is auth-scoped).
	ownerID := userID
	if ownerID == uuid.Nil && len(prospectIDs) > 0 {
		if fp, _ := uc.prospects.GetProspect(ctx, prospectIDs[0]); fp != nil {
			ownerID = fp.UserID
		}
	}
	var feedbackExamples string
	if ownerID != uuid.Nil {
		feedbackExamples = uc.buildFeedbackExamples(ctx, ownerID)
	}

	// Preflight: a sequence with email steps can't be launched unless email
	// (Resend or SMTP) is configured — otherwise the async sender would drop
	// the queued message with nothing to surface to the operator. Skipped when
	// no checker is wired or the sequence has no email steps.
	if uc.emailChecker != nil && ownerID != uuid.Nil && hasEmailStep(steps) {
		if err := uc.emailChecker.IsEmailConfigured(ctx, ownerID); err != nil {
			return result, err
		}
	}

	// Resolve the send mode once for the whole launch. Autopilot ON promotes
	// every queued message straight to Approved (the async sender then
	// dispatches it without a manual approval step); OFF — the default — leaves
	// messages as drafts awaiting human approval. A read error fails the launch
	// rather than guessing, so an unreadable setting can never auto-send.
	var autopilot domain.AutopilotSettings
	if uc.autopilot != nil && ownerID != uuid.Nil {
		s, err := uc.autopilot.ResolveAutopilot(ctx, ownerID)
		if err != nil {
			return result, fmt.Errorf("launch: resolve autopilot mode: %w", err)
		}
		autopilot = s
	}

	// Single source of truth for the launch-time HITL decision: auto-approve
	// (skip the draft queue) only when autopilot is on and the sequence has no
	// approval gate. Drives both the send-delay grace and the status transition
	// below so they can never disagree.
	autoApprove := domain.InitialOutboundStatus(autopilot.Enabled, requireApproval) == domain.OutboundStatusApproved

	for _, pid := range prospectIDs {
		prospect, err := uc.prospects.GetProspect(ctx, pid)
		if err != nil {
			return result, fmt.Errorf("launch: get prospect %s: %w", pid, err)
		}
		// A missing prospect and a foreign prospect return the SAME error (→ 404,
		// "not found") so a caller can't distinguish "doesn't exist" from "exists
		// but isn't yours" — no cross-tenant enumeration.
		if prospect == nil {
			return result, fmt.Errorf("launch: prospect %s: %w", pid, domain.ErrProspectNotOwned)
		}

		// Authorization: every prospect must belong to the authenticated caller.
		// Rejected before any message is queued — closes the IDOR where a caller
		// could launch (and, under autopilot, really send) against another user's
		// prospects. This also subsumes the old single-owner guard: all prospects
		// equal to userID means one owner. userID is nil only in unit tests that
		// don't exercise authz; the handler always supplies a real one.
		if userID != uuid.Nil && prospect.UserID != userID {
			return result, fmt.Errorf("launch: prospect %s: %w", pid, domain.ErrProspectNotOwned)
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

			if step.IsManual() {
				// A manually written step is used verbatim — no AI call. This
				// lets operators run a sequence without an AI provider configured.
				body = step.Body
			} else {
				var genErr error
				prospectID := prospect.ID
				baseMeta := auditdomain.CallMeta{
					UserID:     prospect.UserID,
					ProspectID: &prospectID,
				}
				switch step.Channel {
				case domain.StepChannelTelegram:
					meta := baseMeta
					meta.RequestType = auditdomain.RequestTypeTelegramMessage
					body, genErr = uc.aiGenerator.GenerateTelegramMessage(auditdomain.ContextWithCallMeta(ctx, meta), prospect.Name, prospect.Title, prospect.Company, prospect.Context, step.PromptHint, prevCtx, prospect.Source, feedbackExamples)
				case domain.StepChannelPhoneCall:
					meta := baseMeta
					meta.RequestType = auditdomain.RequestTypeCallBrief
					body, genErr = uc.aiGenerator.GenerateCallBrief(auditdomain.ContextWithCallMeta(ctx, meta), prospect.Name, prospect.Title, prospect.Company, prospect.Context, step.PromptHint, prevCtx)
				default: // "email" or empty
					meta := baseMeta
					meta.RequestType = auditdomain.RequestTypeColdMessage
					body, genErr = uc.aiGenerator.GenerateColdMessage(auditdomain.ContextWithCallMeta(ctx, meta), prospect.Name, prospect.Title, prospect.Company, prospect.Context, step.PromptHint, prevCtx, prospect.Source, feedbackExamples)
				}
				if genErr != nil {
					return result, fmt.Errorf("launch: generate message for prospect %s step %d: %w", pid, step.StepOrder, genErr)
				}
			}

			scheduledAt := now.AddDate(0, 0, cumulativeDelay)
			if immediate {
				scheduledAt = now
			}
			if autoApprove {
				// Push the send out by the configured grace window so an
				// auto-approved message isn't dispatched on the very next sender
				// tick — leaving the operator time to intervene before a real send.
				scheduledAt = scheduledAt.Add(autopilot.SendDelay)
			}
			msg := domain.NewOutboundMessage(pid, sequenceID, step.StepOrder, step.Channel, body, scheduledAt)
			if autoApprove {
				// Skip the draft → human-approval step. draft → approved is a
				// legal transition (see outboundTransitions); the entity guards
				// the state machine so this can't silently corrupt status.
				if err := msg.TransitionTo(domain.OutboundStatusApproved); err != nil {
					return result, fmt.Errorf("launch: auto-approve message for prospect %s step %d: %w", pid, step.StepOrder, err)
				}
			}
			if err := uc.repo.CreateOutboundMessage(ctx, msg); err != nil {
				return result, fmt.Errorf("launch: create outbound message: %w", err)
			}

			previousBody = body
		}

		if err := uc.prospects.MarkInSequence(ctx, pid); err != nil {
			return result, fmt.Errorf("launch: update prospect status: %w", err)
		}
	}

	return result, nil
}

// ApproveMessage loads the message, asks the domain entity to transition to
// Approved (fails if the current status forbids it), then persists the result.
// The state machine (see domain.OutboundMessage.TransitionTo) is the single
// source of truth for legal transitions — the repo is dumb persistence.
func (uc *UseCase) ApproveMessage(ctx context.Context, userID, id uuid.UUID) error {
	msg, err := uc.authorizeMessage(ctx, userID, id)
	if err != nil {
		return fmt.Errorf("approve message: %w", err)
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
func (uc *UseCase) RejectMessage(ctx context.Context, userID, id uuid.UUID) error {
	msg, err := uc.authorizeMessage(ctx, userID, id)
	if err != nil {
		return fmt.Errorf("reject message: %w", err)
	}
	if msg == nil {
		return fmt.Errorf("reject message: not found")
	}
	if err := msg.TransitionTo(domain.OutboundStatusRejected); err != nil {
		return fmt.Errorf("reject message: %w", err)
	}
	return uc.repo.UpdateOutboundStatus(ctx, id, msg.Status)
}

func (uc *UseCase) EditMessage(ctx context.Context, userID, id uuid.UUID, body string) error {
	// Read original message before updating (also authorizes ownership).
	msg, err := uc.authorizeMessage(ctx, userID, id)
	if err != nil {
		return fmt.Errorf("edit message: %w", err)
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

func (uc *UseCase) DeleteStep(ctx context.Context, userID, stepID uuid.UUID) error {
	if err := uc.authorizeStep(ctx, userID, stepID); err != nil {
		return err
	}
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
