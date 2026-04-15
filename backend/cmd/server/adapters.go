package main

import (
	"context"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/chat"
	"github.com/daniil/floq/internal/inbox"
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
	repo prospectsdomain.Repository
}

func newProspectRepoAdapter(repo prospectsdomain.Repository) inbox.ProspectRepository {
	return &prospectRepoAdapter{repo: repo}
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

func (a *prospectRepoAdapter) ConvertToLead(ctx context.Context, prospectID, leadID uuid.UUID) error {
	return a.repo.ConvertToLead(ctx, prospectID, leadID)
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

func fromInboxQualification(q *inbox.InboxQualification) *leadsdomain.Qualification {
	if q == nil {
		return nil
	}
	return &leadsdomain.Qualification{
		ID:                q.ID,
		LeadID:            q.LeadID,
		IdentifiedNeed:    q.IdentifiedNeed,
		EstimatedBudget:   q.EstimatedBudget,
		Deadline:          q.Deadline,
		Score:             q.Score,
		ScoreReason:       q.ScoreReason,
		RecommendedAction: q.RecommendedAction,
		ProviderUsed:      q.ProviderUsed,
		GeneratedAt:       q.GeneratedAt,
	}
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

