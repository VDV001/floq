package leads

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	auditdomain "github.com/daniil/floq/internal/audit/domain"
	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
)

type UseCase struct {
	repo             domain.Repository
	ai               domain.AIService
	sender           domain.MessageSender
	suggestionFinder domain.ProspectSuggestionFinder
	identityReader   IdentityReader
	pendingCounter   PendingReplyCounter
	qualObserver     domain.QualificationObserver
	logger           *slog.Logger
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

// WithQualificationObserver wires the post-qualification hook (issue #108).
// Supplied from the composition root via an adapter that bridges to the onec
// context. nil leaves the hook disabled.
func WithQualificationObserver(o domain.QualificationObserver) Option {
	return func(uc *UseCase) { uc.qualObserver = o }
}

// WithLogger overrides the default slog.Logger so the use case emits
// structured warnings through the server-wide handler. Pass nil to
// keep slog.Default().
func WithLogger(l *slog.Logger) Option {
	return func(uc *UseCase) {
		if l != nil {
			uc.logger = l
		}
	}
}

func NewUseCase(repo domain.Repository, ai domain.AIService, sender domain.MessageSender, opts ...Option) *UseCase {
	uc := &UseCase{repo: repo, ai: ai, sender: sender, logger: slog.Default()}
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

// SetQualificationObserver wires the post-qualification hook after construction,
// needed because the onec outbound use case (which the observer bridges to) is
// built later in the composition root than the leads use case.
func (uc *UseCase) SetQualificationObserver(o domain.QualificationObserver) {
	uc.qualObserver = o
}

func (uc *UseCase) ListLeads(ctx context.Context, userID uuid.UUID) ([]domain.LeadWithSource, error) {
	return uc.repo.ListLeads(ctx, userID)
}

// ListArchivedLeads returns the user's archived leads (newest-archived first)
// for the dedicated archive view. Active leads live in ListLeads.
func (uc *UseCase) ListArchivedLeads(ctx context.Context, userID uuid.UUID) ([]domain.LeadWithSource, error) {
	return uc.repo.ListArchivedLeads(ctx, userID)
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
		return domain.ErrLeadNotFound
	}

	if err := lead.TransitionTo(target); err != nil {
		return err
	}

	return uc.repo.UpdateLeadStatus(ctx, id, target)
}

// ArchiveLead hides the lead from working feeds and analytics without touching
// its pipeline status. Ownership is enforced upstream by the handler's
// authorizeLead front-door (the shared gate for every /api/leads/{id}/* route),
// mirroring UpdateStatus. The archive invariant (reject double-archive) lives on
// Lead.Archive; a re-archive surfaces domain.ErrAlreadyArchived for a 409.
func (uc *UseCase) ArchiveLead(ctx context.Context, id uuid.UUID) error {
	lead, err := uc.repo.GetLead(ctx, id)
	if err != nil {
		return fmt.Errorf("get lead: %w", err)
	}
	if lead == nil {
		return domain.ErrLeadNotFound
	}
	if err := lead.Archive(); err != nil {
		return err
	}
	return uc.repo.SetLeadArchived(ctx, id, lead.ArchivedAt)
}

// UnarchiveLead restores an archived lead to feeds and analytics. Returns
// domain.ErrNotArchived when the lead is not archived.
func (uc *UseCase) UnarchiveLead(ctx context.Context, id uuid.UUID) error {
	lead, err := uc.repo.GetLead(ctx, id)
	if err != nil {
		return fmt.Errorf("get lead: %w", err)
	}
	if lead == nil {
		return domain.ErrLeadNotFound
	}
	if err := lead.Unarchive(); err != nil {
		return err
	}
	return uc.repo.SetLeadArchived(ctx, id, lead.ArchivedAt)
}

func (uc *UseCase) GetMessages(ctx context.Context, leadID uuid.UUID) ([]domain.Message, error) {
	return uc.repo.ListMessages(ctx, leadID)
}

// OwnsLead returns true iff the lead exists AND belongs to userID. The
// (nil-lead, nil-error) shape from GetLeadForUser maps to false here so
// handlers can branch on a single boolean. Errors propagate.
func (uc *UseCase) OwnsLead(ctx context.Context, userID, leadID uuid.UUID) (bool, error) {
	lead, err := uc.repo.GetLeadForUser(ctx, userID, leadID)
	if err != nil {
		return false, err
	}
	return lead != nil, nil
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

	auditLeadID := lead.ID
	auditCtx := auditdomain.ContextWithCallMeta(ctx, auditdomain.CallMeta{
		UserID:      lead.UserID,
		LeadID:      &auditLeadID,
		RequestType: auditdomain.RequestTypeQualification,
	})
	aiResult, err := uc.ai.Qualify(auditCtx, lead.ContactName, lead.Channel, lead.FirstMessage)
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
	lead.Status = domain.StatusQualified

	// Fire the post-qualification side-effect (e.g. push a counterparty to 1C).
	// The observer owns its errors — a failure here must not fail qualification.
	if uc.qualObserver != nil {
		uc.qualObserver.OnLeadQualified(ctx, lead)
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

	auditLeadID := lead.ID
	auditCtx := auditdomain.ContextWithCallMeta(ctx, auditdomain.CallMeta{
		UserID:      lead.UserID,
		LeadID:      &auditLeadID,
		RequestType: auditdomain.RequestTypeDraftReply,
	})
	body, err := uc.ai.DraftReply(auditCtx, lead.ContactName, firstMsg)
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
	// Export is a backup — it must include archived leads (ListAllLeads), not
	// just the active inbox feed, and mark each row's archived state so the
	// file round-trips the full picture.
	leads, err := uc.repo.ListAllLeads(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list leads: %w", err)
	}

	var buf bytes.Buffer
	// BOM for Excel compatibility
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	w := csv.NewWriter(&buf)
	// archived_at carries the FULL archive state (the exact timestamp, empty for
	// active) so a backup round-trips it precisely — not just a boolean, which
	// would reset the archive date to the import day on restore.
	header := []string{"contact_name", "company", "channel", "email_address", "status", "first_message", "created_at", "archived_at"}
	if err := w.Write(header); err != nil {
		return nil, fmt.Errorf("write csv header: %w", err)
	}

	for _, l := range leads {
		emailAddr := ""
		if l.EmailAddress != nil {
			emailAddr = *l.EmailAddress
		}
		archivedAt := ""
		if l.ArchivedAt != nil {
			archivedAt = l.ArchivedAt.Format(time.RFC3339)
		}
		record := []string{
			l.ContactName,
			l.Company,
			string(l.Channel),
			emailAddr,
			string(l.Status),
			l.FirstMessage,
			l.CreatedAt.Format(time.RFC3339),
			archivedAt,
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
	// Strip a leading UTF-8 BOM so the first header name parses as "contact_name"
	// rather than a BOM-prefixed variant. ExportCSV writes a BOM for Excel, so
	// without this our own backup files fail to re-import (missing contact_name).
	csvData = bytes.TrimPrefix(csvData, []byte{0xEF, 0xBB, 0xBF})
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
		// The export's "archived_at" column lets a backup round-trip the exact
		// archive timestamp. Empty (or absent, for a normal user-provided list)
		// reads as active.
		var importedArchivedAt *time.Time
		if raw := getCol(record, "archived_at"); raw != "" {
			if t, perr := time.Parse(time.RFC3339, raw); perr == nil {
				utc := t.UTC()
				importedArchivedAt = &utc
			}
		}

		var emailPtr *string
		if emailAddr != "" {
			existing, err := uc.repo.GetLeadByEmailAddress(ctx, userID, emailAddr)
			if err != nil {
				return 0, fmt.Errorf("dedup lead check: %w", err)
			}
			if existing != nil {
				// Re-importing an *active* row resurfaces a matched archived lead —
				// that reads as re-engagement, not lost data. A row that is itself
				// archived leaves the existing state untouched; an active duplicate
				// is a true dedup hit. SetLeadArchived(nil) is the guarded unarchive;
				// swallow ErrNotArchived in case a concurrent inbound already did it.
				if existing.IsArchived() && importedArchivedAt == nil {
					if err := uc.repo.SetLeadArchived(ctx, existing.ID, nil); err != nil && !errors.Is(err, domain.ErrNotArchived) {
						return 0, fmt.Errorf("persist resurface: %w", err)
					}
					count++
				}
				continue
			}
			emailPtr = &emailAddr
		}

		lead, err := domain.NewLead(userID, channel, contactName, company, firstMessage, nil, emailPtr)
		if err != nil {
			continue // skip invalid rows
		}
		// Rehydrate the exact archive timestamp from the backup so a restore
		// preserves history instead of flooding archived leads back as active
		// (CreateLead persists archived_at).
		lead.ArchivedAt = importedArchivedAt
		if err := uc.repo.CreateLead(ctx, lead); err != nil {
			return 0, fmt.Errorf("create lead: %w", err)
		}
		count++
	}

	return count, nil
}
