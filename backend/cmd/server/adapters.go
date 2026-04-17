package main

import (
	"context"
	"fmt"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/chat"
	"github.com/daniil/floq/internal/db"
	"github.com/daniil/floq/internal/inbox"
	"github.com/daniil/floq/internal/leads"
	leadsdomain "github.com/daniil/floq/internal/leads/domain"
	"github.com/daniil/floq/internal/prospects"
	prospectsdomain "github.com/daniil/floq/internal/prospects/domain"
	settingsdomain "github.com/daniil/floq/internal/settings/domain"
	"github.com/google/uuid"
)

// --- LeadChecker adapter (prospects → leads boundary) ---

type leadCheckerAdapter struct {
	repo leadsdomain.Repository
}

func newLeadCheckerAdapter(repo leadsdomain.Repository) prospects.LeadChecker {
	return &leadCheckerAdapter{repo: repo}
}

func (a *leadCheckerAdapter) LeadExistsByEmail(ctx context.Context, userID uuid.UUID, email string) (bool, error) {
	lead, err := a.repo.GetLeadByEmailAddress(ctx, userID, email)
	if err != nil {
		return false, err
	}
	return lead != nil, nil
}

// --- ProspectRepo adapter (inbox → prospects boundary) ---

type prospectRepoAdapter struct {
	repo      prospectsdomain.Repository
	txManager *db.TxManager
}

func newProspectRepoAdapter(repo prospectsdomain.Repository, txManager *db.TxManager) inbox.ProspectRepository {
	return &prospectRepoAdapter{repo: repo, txManager: txManager}
}

func toProspectMatch(p *prospectsdomain.Prospect) *inbox.ProspectMatch {
	if p == nil {
		return nil
	}
	return &inbox.ProspectMatch{
		ID:       p.ID,
		Name:     p.Name,
		Company:  p.Company,
		SourceID: p.SourceID,
		Status:   string(p.Status),
	}
}

func (a *prospectRepoAdapter) FindByEmail(ctx context.Context, userID uuid.UUID, email string) (*inbox.ProspectMatch, error) {
	p, err := a.repo.FindByEmail(ctx, userID, email)
	if err != nil {
		return nil, err
	}
	return toProspectMatch(p), nil
}

func (a *prospectRepoAdapter) FindByTelegramUsername(ctx context.Context, userID uuid.UUID, username string) (*inbox.ProspectMatch, error) {
	p, err := a.repo.FindByTelegramUsername(ctx, userID, username)
	if err != nil {
		return nil, err
	}
	return toProspectMatch(p), nil
}

// ConvertToLead routes through the Prospect domain entity rather than firing
// raw SQL: the adapter loads the prospect, calls MarkConvertedToLead (which
// enforces the state-machine rule "not already terminal"), and persists the
// resulting status+converted_lead_id in a single transaction. This closes the
// "domain method exists but bypass path exists" gap — inbox callers
// (email.go, telegram.go) cannot accidentally double-convert a prospect.
func (a *prospectRepoAdapter) ConvertToLead(ctx context.Context, prospectID, leadID uuid.UUID) error {
	return a.txManager.WithTx(ctx, func(txCtx context.Context) error {
		p, err := a.repo.GetProspect(txCtx, prospectID)
		if err != nil {
			return err
		}
		if p == nil {
			return fmt.Errorf("convert to lead: prospect not found")
		}
		if err := p.MarkConvertedToLead(leadID); err != nil {
			return fmt.Errorf("convert to lead: %w", err)
		}
		return a.repo.ConvertToLead(txCtx, p.ID, leadID)
	})
}

// --- InboxLeadRepo adapter (inbox → leads boundary) ---

type inboxLeadRepoAdapter struct {
	repo leadsdomain.Repository
}

func newInboxLeadRepoAdapter(repo leadsdomain.Repository) inbox.LeadRepository {
	return &inboxLeadRepoAdapter{repo: repo}
}

func (a *inboxLeadRepoAdapter) GetLeadByTelegramChatID(ctx context.Context, userID uuid.UUID, chatID int64) (*inbox.InboxLead, error) {
	lead, err := a.repo.GetLeadByTelegramChatID(ctx, userID, chatID)
	if err != nil {
		return nil, err
	}
	return toInboxLead(lead), nil
}

func (a *inboxLeadRepoAdapter) GetLeadByEmailAddress(ctx context.Context, userID uuid.UUID, email string) (*inbox.InboxLead, error) {
	lead, err := a.repo.GetLeadByEmailAddress(ctx, userID, email)
	if err != nil {
		return nil, err
	}
	return toInboxLead(lead), nil
}

func (a *inboxLeadRepoAdapter) CreateLead(ctx context.Context, lead *inbox.InboxLead) error {
	domainLead := fromInboxLead(lead)
	return a.repo.CreateLead(ctx, domainLead)
}

func (a *inboxLeadRepoAdapter) UpdateFirstMessage(ctx context.Context, id uuid.UUID, message string) error {
	return a.repo.UpdateFirstMessage(ctx, id, message)
}

func (a *inboxLeadRepoAdapter) CreateMessage(ctx context.Context, msg *inbox.InboxMessage) error {
	domainMsg := fromInboxMessage(msg)
	return a.repo.CreateMessage(ctx, domainMsg)
}

func (a *inboxLeadRepoAdapter) UpsertQualification(ctx context.Context, q *inbox.InboxQualification) error {
	domainQ := fromInboxQualification(q)
	return a.repo.UpsertQualification(ctx, domainQ)
}

func (a *inboxLeadRepoAdapter) UpdateLeadStatus(ctx context.Context, id uuid.UUID, status inbox.LeadStatus) error {
	return a.repo.UpdateLeadStatus(ctx, id, leadsdomain.LeadStatus(status))
}

// --- InboxAI adapter (inbox → ai boundary) ---

type inboxAIAdapter struct {
	client *ai.AIClient
}

func newInboxAIAdapter(client *ai.AIClient) inbox.AIQualifier {
	return &inboxAIAdapter{client: client}
}

func (a *inboxAIAdapter) Qualify(ctx context.Context, contactName, channel, firstMessage string) (*inbox.QualificationResult, error) {
	result, err := a.client.Qualify(ctx, contactName, channel, firstMessage)
	if err != nil {
		return nil, err
	}
	return &inbox.QualificationResult{
		IdentifiedNeed:    result.IdentifiedNeed,
		EstimatedBudget:   result.EstimatedBudget,
		Deadline:          result.Deadline,
		Score:             result.Score,
		ScoreReason:       result.ScoreReason,
		RecommendedAction: result.RecommendedAction,
	}, nil
}

func (a *inboxAIAdapter) ProviderName() string {
	return a.client.ProviderName()
}

// --- InboxConfig adapter (inbox → settings boundary) ---

type inboxConfigAdapter struct {
	store interface {
		GetConfig(ctx context.Context, userID uuid.UUID) (*settingsdomain.UserConfig, error)
	}
}

func newInboxConfigAdapter(store interface {
	GetConfig(ctx context.Context, userID uuid.UUID) (*settingsdomain.UserConfig, error)
}) inbox.ConfigStore {
	return &inboxConfigAdapter{store: store}
}

func (a *inboxConfigAdapter) GetConfig(ctx context.Context, userID uuid.UUID) (*inbox.InboxConfig, error) {
	cfg, err := a.store.GetConfig(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &inbox.InboxConfig{
		IMAPHost:         cfg.IMAPHost,
		IMAPPort:         cfg.IMAPPort,
		IMAPUser:         cfg.IMAPUser,
		IMAPPassword:     cfg.IMAPPassword,
		TelegramBotToken: cfg.TelegramBotToken,
	}, nil
}

// --- Mapping helpers ---

func toInboxLead(lead *leadsdomain.Lead) *inbox.InboxLead {
	if lead == nil {
		return nil
	}
	return &inbox.InboxLead{
		ID:             lead.ID,
		UserID:         lead.UserID,
		Channel:        inbox.Channel(lead.Channel),
		ContactName:    lead.ContactName,
		Company:        lead.Company,
		FirstMessage:   lead.FirstMessage,
		Status:         inbox.LeadStatus(lead.Status),
		TelegramChatID: lead.TelegramChatID,
		EmailAddress:   lead.EmailAddress,
		SourceID:       lead.SourceID,
		CreatedAt:      lead.CreatedAt,
		UpdatedAt:      lead.UpdatedAt,
	}
}

func fromInboxLead(lead *inbox.InboxLead) *leadsdomain.Lead {
	if lead == nil {
		return nil
	}
	return &leadsdomain.Lead{
		ID:             lead.ID,
		UserID:         lead.UserID,
		Channel:        leadsdomain.Channel(lead.Channel),
		ContactName:    lead.ContactName,
		Company:        lead.Company,
		FirstMessage:   lead.FirstMessage,
		Status:         leadsdomain.LeadStatus(lead.Status),
		TelegramChatID: lead.TelegramChatID,
		EmailAddress:   lead.EmailAddress,
		SourceID:       lead.SourceID,
		CreatedAt:      lead.CreatedAt,
		UpdatedAt:      lead.UpdatedAt,
	}
}

func fromInboxMessage(msg *inbox.InboxMessage) *leadsdomain.Message {
	if msg == nil {
		return nil
	}
	return &leadsdomain.Message{
		ID:        msg.ID,
		LeadID:    msg.LeadID,
		Direction: leadsdomain.MessageDirection(msg.Direction),
		Body:      msg.Body,
		SentAt:    msg.SentAt,
	}
}

// fromInboxQualification rehydrates a Qualification from the inbox's local
// DTO via the domain-owned factory RehydrateQualification, which enforces
// the score [0,100] invariant at construction. The adapter no longer
// carries a struct literal — the only way to build a leads Qualification
// at this boundary goes through a domain constructor.
func fromInboxQualification(q *inbox.InboxQualification) *leadsdomain.Qualification {
	if q == nil {
		return nil
	}
	return leadsdomain.RehydrateQualification(
		q.ID, q.LeadID,
		q.IdentifiedNeed, q.EstimatedBudget, q.Deadline,
		q.Score, q.ScoreReason, q.RecommendedAction, q.ProviderUsed,
		q.GeneratedAt,
	)
}

// --- ProspectSuggestionFinder adapter (leads → prospects boundary for cross-channel dedup) ---
//
// Implements leadsdomain.ProspectSuggestionFinder. The adapter owns cross-
// context orchestration for issue #6: SQL matcher (3 tiers) is in
// prospects.Repository.FindSuggestionsForLead; Link runs in a single
// db.TxManager transaction that spans both contexts' repositories, calling
// the Lead.InheritsSourceFrom domain rule + Lead.SetSource mutator to
// propagate source_id according to the entity's policy; every mutation
// verifies ownership (user_id) via repo.*ForUser methods and returns
// ErrLeadNotFound / ErrProspectNotFound on mismatch — "not yours" and
// "doesn't exist" collapse into the same sentinel by design.

// Compile-time proof the adapter satisfies the leads-domain port.
var _ leadsdomain.ProspectSuggestionFinder = (*prospectSuggestionFinderAdapter)(nil)

type prospectSuggestionFinderAdapter struct {
	txManager     *db.TxManager
	leadsRepo     *leads.Repository
	prospectsRepo *prospects.Repository
}

func newProspectSuggestionFinderAdapter(
	txManager *db.TxManager,
	leadsRepo *leads.Repository,
	prospectsRepo *prospects.Repository,
) leadsdomain.ProspectSuggestionFinder {
	return &prospectSuggestionFinderAdapter{
		txManager:     txManager,
		leadsRepo:     leadsRepo,
		prospectsRepo: prospectsRepo,
	}
}

func (a *prospectSuggestionFinderAdapter) loadLeadForUser(ctx context.Context, userID, leadID uuid.UUID) (*leadsdomain.Lead, error) {
	lead, err := a.leadsRepo.GetLeadForUser(ctx, userID, leadID)
	if err != nil {
		return nil, err
	}
	if lead == nil {
		return nil, leadsdomain.ErrLeadNotFound
	}
	return lead, nil
}

func (a *prospectSuggestionFinderAdapter) loadProspectForUser(ctx context.Context, userID, prospectID uuid.UUID) (*prospectsdomain.Prospect, error) {
	p, err := a.prospectsRepo.GetProspectForUser(ctx, userID, prospectID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, leadsdomain.ErrProspectNotFound
	}
	return p, nil
}

func (a *prospectSuggestionFinderAdapter) FindForLead(ctx context.Context, userID, leadID uuid.UUID) ([]leadsdomain.ProspectSuggestion, error) {
	lead, err := a.loadLeadForUser(ctx, userID, leadID)
	if err != nil {
		return nil, err
	}
	email := ""
	if lead.EmailAddress != nil {
		email = *lead.EmailAddress
	}
	rows, err := a.prospectsRepo.FindSuggestionsForLead(ctx, userID, lead.ID, lead.ContactName, lead.Company, email)
	if err != nil {
		return nil, err
	}
	out := make([]leadsdomain.ProspectSuggestion, 0, len(rows))
	for _, r := range rows {
		out = append(out, leadsdomain.ProspectSuggestion{
			ProspectID:       r.ProspectID,
			Name:             r.Name,
			Company:          r.Company,
			Email:            r.Email,
			TelegramUsername: r.TelegramUsername,
			SourceName:       r.SourceName,
			Status:           r.Status,
			Confidence:       leadsdomain.SuggestionConfidence(r.Confidence),
		})
	}
	return out, nil
}

func (a *prospectSuggestionFinderAdapter) CountsForUser(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]int, error) {
	return a.prospectsRepo.CountSuggestionsForUser(ctx, userID)
}

// LinkProspect runs the link inside a single transaction, routed through
// each context's own repository (db.ConnFromCtx routes Query/Exec onto
// the shared tx). No raw SQL here — the adapter is pure orchestration.
func (a *prospectSuggestionFinderAdapter) LinkProspect(ctx context.Context, userID, leadID, prospectID uuid.UUID) error {
	return a.txManager.WithTx(ctx, func(txCtx context.Context) error {
		lead, err := a.loadLeadForUser(txCtx, userID, leadID)
		if err != nil {
			return err
		}
		prospect, err := a.loadProspectForUser(txCtx, userID, prospectID)
		if err != nil {
			return err
		}

		// Mark prospect as converted to this lead.
		if err := a.prospectsRepo.ConvertToLead(txCtx, prospect.ID, lead.ID); err != nil {
			return err
		}

		// Apply the Lead's source-inheritance rule. InheritsSourceFrom
		// returns (newSource, changed) — we persist only when changed is
		// true, so no pointer-equality guesswork here.
		newSource, changed := lead.InheritsSourceFrom(prospect.SourceID)
		if !changed {
			return nil
		}
		lead.SetSource(newSource)
		return a.leadsRepo.UpdateSourceID(txCtx, lead.ID, lead.SourceID)
	})
}

func (a *prospectSuggestionFinderAdapter) DismissSuggestion(ctx context.Context, userID, leadID, prospectID uuid.UUID) error {
	// Verify ownership of both sides before persisting a dismissal —
	// prevents cross-tenant IDs from leaking into the dismissals table.
	if _, err := a.loadLeadForUser(ctx, userID, leadID); err != nil {
		return err
	}
	if _, err := a.loadProspectForUser(ctx, userID, prospectID); err != nil {
		return err
	}
	return a.prospectsRepo.DismissSuggestion(ctx, leadID, prospectID)
}

// --- Chat AI adapter (chat → ai boundary) ---

type chatAIAdapter struct {
	client *ai.AIClient
}

func newChatAIAdapter(client *ai.AIClient) chat.AIClient {
	return &chatAIAdapter{client: client}
}

func (a *chatAIAdapter) Complete(ctx context.Context, req chat.ChatCompletionRequest) (string, error) {
	msgs := make([]ai.Message, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = ai.Message{Role: m.Role, Content: m.Content}
	}
	return a.client.Complete(ctx, ai.CompletionRequest{
		Messages:  msgs,
		MaxTokens: req.MaxTokens,
	})
}

func (a *chatAIAdapter) ProviderName() string {
	return a.client.ProviderName()
}

