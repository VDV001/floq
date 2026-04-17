package leads

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
)

type UseCase struct {
	repo             domain.Repository
	ai               domain.AIService
	sender           domain.MessageSender
	suggestionFinder domain.ProspectSuggestionFinder
}

// Option configures a *UseCase at construction. Used for dependencies that
// are optional or that cross context boundaries (injected via adapter).
type Option func(*UseCase)

// WithSuggestionFinder wires the cross-channel prospect-suggestion port
// (issue #6). Typically supplied from the composition root after the
// adapter is built.
func WithSuggestionFinder(f domain.ProspectSuggestionFinder) Option {
	return func(uc *UseCase) { uc.suggestionFinder = f }
}

func NewUseCase(repo domain.Repository, ai domain.AIService, sender domain.MessageSender, opts ...Option) *UseCase {
	uc := &UseCase{repo: repo, ai: ai, sender: sender}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

// SetSender sets the message sender after construction (e.g. when the Telegram bot
// is initialised later than the use case).
func (uc *UseCase) SetSender(sender domain.MessageSender) {
	uc.sender = sender
}

func (uc *UseCase) ListLeads(ctx context.Context, userID uuid.UUID) ([]domain.LeadWithSource, error) {
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

	// Auto-transition rule lives on the entity (Lead.OnOutboundSent) — the
	// usecase just orchestrates persistence. No lead.Status check at this
	// layer; the domain method decides whether a transition applies.
	if lead.OnOutboundSent() {
		if err := uc.repo.UpdateLeadStatus(ctx, leadID, lead.Status); err != nil {
			return nil, fmt.Errorf("persist auto-transition: %w", err)
		}
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

	aiResult, err := uc.ai.Qualify(ctx, lead.ContactName, lead.Channel, lead.FirstMessage)
	if err != nil {
		return nil, err
	}

	q := domain.NewQualification(lead.ID, aiResult.IdentifiedNeed, aiResult.EstimatedBudget, aiResult.Deadline, aiResult.Score, aiResult.ScoreReason, aiResult.RecommendedAction, aiResult.ProviderUsed)
	if err := uc.repo.UpsertQualification(ctx, q); err != nil {
		return nil, err
	}

	if err := lead.TransitionTo(domain.StatusQualified); err != nil {
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

	d, err := domain.NewDraft(lead.ID, body)
	if err != nil {
		return nil, fmt.Errorf("construct draft: %w", err)
	}

	if err := uc.repo.CreateDraft(ctx, d); err != nil {
		return nil, err
	}

	return d, nil
}

func (uc *UseCase) ExportCSV(ctx context.Context, userID uuid.UUID) ([]byte, error) {
	leads, err := uc.repo.ListLeads(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list leads: %w", err)
	}

	var buf bytes.Buffer
	// BOM for Excel compatibility
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	w := csv.NewWriter(&buf)
	header := []string{"contact_name", "company", "channel", "email_address", "status", "first_message", "created_at"}
	if err := w.Write(header); err != nil {
		return nil, fmt.Errorf("write csv header: %w", err)
	}

	for _, l := range leads {
		emailAddr := ""
		if l.EmailAddress != nil {
			emailAddr = *l.EmailAddress
		}
		record := []string{
			l.ContactName,
			l.Company,
			string(l.Channel),
			emailAddr,
			string(l.Status),
			l.FirstMessage,
			l.CreatedAt.Format(time.RFC3339),
		}
		if err := w.Write(record); err != nil {
			return nil, fmt.Errorf("write csv record: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("flush csv: %w", err)
	}

	return buf.Bytes(), nil
}

func (uc *UseCase) ImportCSV(ctx context.Context, userID uuid.UUID, csvData []byte) (int, error) {
	reader := csv.NewReader(bytes.NewReader(csvData))

	// Read and validate header
	header, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("read csv header: %w", err)
	}

	colIndex := make(map[string]int, len(header))
	for i, name := range header {
		colIndex[name] = i
	}

	// Validate required columns
	if _, ok := colIndex["contact_name"]; !ok {
		return 0, fmt.Errorf("missing required column: contact_name")
	}
	if _, ok := colIndex["channel"]; !ok {
		return 0, fmt.Errorf("missing required column: channel")
	}

	getCol := func(record []string, name string) string {
		if idx, ok := colIndex[name]; ok && idx < len(record) {
			return record[idx]
		}
		return ""
	}

	var count int
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("read csv record: %w", err)
		}

		contactName := getCol(record, "contact_name")
		if contactName == "" {
			continue
		}

		ch := getCol(record, "channel")
		channel := domain.ChannelEmail
		if ch == "telegram" {
			channel = domain.ChannelTelegram
		}

		company := getCol(record, "company")
		firstMessage := getCol(record, "first_message")
		emailAddr := getCol(record, "email_address")

		var emailPtr *string
		if emailAddr != "" {
			existing, err := uc.repo.GetLeadByEmailAddress(ctx, userID, emailAddr)
			if err != nil {
				return 0, fmt.Errorf("dedup lead check: %w", err)
			}
			if existing != nil {
				continue
			}
			emailPtr = &emailAddr
		}

		lead, err := domain.NewLead(userID, channel, contactName, company, firstMessage, nil, emailPtr)
		if err != nil {
			continue // skip invalid rows
		}
		if err := uc.repo.CreateLead(ctx, lead); err != nil {
			return 0, fmt.Errorf("create lead: %w", err)
		}
		count++
	}

	return count, nil
}
