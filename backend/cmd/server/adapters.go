package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/ai/security"
	"github.com/daniil/floq/internal/chat"
	"github.com/daniil/floq/internal/db"
	"github.com/daniil/floq/internal/inbox"
	"github.com/daniil/floq/internal/integrations/onec"
	onecdomain "github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/daniil/floq/internal/leads"
	leadsdomain "github.com/daniil/floq/internal/leads/domain"
	"github.com/daniil/floq/internal/outbound"
	"github.com/daniil/floq/internal/prospects"
	prospectsdomain "github.com/daniil/floq/internal/prospects/domain"
	settingsdomain "github.com/daniil/floq/internal/settings/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time check that *leads.UseCase satisfies the structural
// port inbox.LeadOwnershipChecker. Pinned here so a future signature
// drift on leads.UseCase.OwnsLead breaks the build at the wiring
// edge instead of silently breaking the structural conformance
// inbox.RegisterPendingReplyRoutes relies on.
var _ inbox.LeadOwnershipChecker = (*leads.UseCase)(nil)

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
		if err := a.repo.ConvertToLead(txCtx, p.ID, leadID); err != nil {
			return err
		}
		// An inbound reply is the prospect engaging with us — the legitimate
		// basis for contact. Record obtained consent in the same transaction so
		// future outbound to them needs no cold-contact override.
		//
		// Withdrawal is the absolute red line: a prospect who opted out and then
		// writes (about anything) must NOT be silently resurrected to obtained.
		// Lifting a withdrawal is a deliberate fresh opt-in (the manual toggle),
		// never an automatic side effect of a reply.
		if p.Consent.Status == prospectsdomain.ConsentStatusWithdrawn {
			return nil
		}
		if err := p.GrantConsent(prospectsdomain.ConsentSourceInboundReply, time.Now().UTC()); err != nil {
			return fmt.Errorf("grant inbound consent: %w", err)
		}
		return a.repo.UpdateConsent(txCtx, p.ID, p.Consent)
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

// --- InboxInputClassifier adapter (inbox → ai/security boundary) ---
//
// Adapts security.InputFirewall to the inbox.InputClassifier port so the
// inbox context can stamp a reply's InputSeverity without importing
// internal/ai/security. The severity-ladder translation lives here, at the
// boundary — the only place that legitimately knows both vocabularies.

type inboxInputClassifier struct {
	firewall *security.InputFirewall
}

func newInboxInputClassifier(firewall *security.InputFirewall) inbox.InputClassifier {
	return &inboxInputClassifier{firewall: firewall}
}

func (c *inboxInputClassifier) Classify(text string) inbox.Severity {
	return mapSecuritySeverity(c.firewall.Scan(text).Severity)
}

// mapSecuritySeverity translates the security severity ladder onto the
// inbox one. Kept a pure function (no firewall dependency) so the mapping
// is unit-testable in isolation; an unknown value defaults to Info, the
// safe baseline (never escalates an unrecognised verdict to a block).
func mapSecuritySeverity(s security.Severity) inbox.Severity {
	switch s {
	case security.SeverityBlock:
		return inbox.SeverityBlock
	case security.SeverityWarn:
		return inbox.SeverityWarn
	default:
		return inbox.SeverityInfo
	}
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

// --- IdentityLinker adapter (inbox + prospects → leads-identity boundary) ---

// identityLinkerAdapter bridges the narrow IdentityLinker ports the
// inbox + prospects contexts expose to the leads-context machinery.
// Both LinkLeadToIdentity and LinkProspectToIdentity route through one
// resolver, so a lead and a prospect that share an identifier collapse
// to the same Identity row.
type identityLinkerAdapter struct {
	resolver leadsdomain.IdentityResolver
	repo     leadsdomain.IdentityRepository
}

// Compile-time checks: the adapter must satisfy BOTH narrow ports.
var (
	_ inbox.IdentityLinker     = (*identityLinkerAdapter)(nil)
	_ prospects.IdentityLinker = (*identityLinkerAdapter)(nil)
)

func newIdentityLinkerAdapter(resolver leadsdomain.IdentityResolver, repo leadsdomain.IdentityRepository) *identityLinkerAdapter {
	return &identityLinkerAdapter{resolver: resolver, repo: repo}
}

func (a *identityLinkerAdapter) LinkLeadToIdentity(ctx context.Context, userID, leadID uuid.UUID, email, phone, telegramUsername string) error {
	id, err := a.resolver.Resolve(ctx, userID, email, phone, telegramUsername)
	if err != nil {
		return fmt.Errorf("resolve identity for lead: %w", err)
	}
	return a.repo.LinkLead(ctx, leadID, id.ID)
}

func (a *identityLinkerAdapter) LinkProspectToIdentity(ctx context.Context, userID, prospectID uuid.UUID, email, phone, telegramUsername string) error {
	id, err := a.resolver.Resolve(ctx, userID, email, phone, telegramUsername)
	if err != nil {
		return fmt.Errorf("resolve identity for prospect: %w", err)
	}
	return a.repo.LinkProspect(ctx, prospectID, id.ID)
}

// --- SQL BackfillSource for IdentityBackfill ---

// sqlBackfillSource feeds IdentityBackfill from the legacy leads +
// prospects tables. Queries are intentionally simple full scans
// scoped by `... IS NOT NULL` / non-empty filters — backfill is a
// one-shot startup job, not a hot path.
type sqlBackfillSource struct {
	pool *pgxpool.Pool
}

var _ leads.BackfillSource = (*sqlBackfillSource)(nil)

func newSQLBackfillSource(pool *pgxpool.Pool) *sqlBackfillSource {
	return &sqlBackfillSource{pool: pool}
}

func (s *sqlBackfillSource) LeadsForBackfill(ctx context.Context) ([]leads.LeadIdentifierRow, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, COALESCE(email_address, '')
		 FROM leads
		 WHERE email_address IS NOT NULL AND email_address <> ''`)
	if err != nil {
		return nil, fmt.Errorf("backfill source: query leads: %w", err)
	}
	defer rows.Close()
	var out []leads.LeadIdentifierRow
	for rows.Next() {
		var r leads.LeadIdentifierRow
		if err := rows.Scan(&r.LeadID, &r.UserID, &r.Email); err != nil {
			return nil, fmt.Errorf("backfill source: scan lead: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *sqlBackfillSource) ProspectsForBackfill(ctx context.Context) ([]leads.ProspectIdentifierRow, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, email, phone, telegram_username
		 FROM prospects
		 WHERE email <> '' OR phone <> '' OR telegram_username <> ''`)
	if err != nil {
		return nil, fmt.Errorf("backfill source: query prospects: %w", err)
	}
	defer rows.Close()
	var out []leads.ProspectIdentifierRow
	for rows.Next() {
		var r leads.ProspectIdentifierRow
		if err := rows.Scan(&r.ProspectID, &r.UserID, &r.Email, &r.Phone, &r.TelegramUsername); err != nil {
			return nil, fmt.Errorf("backfill source: scan prospect: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// pendingReplyCounterAdapter bridges inbox.PendingReplyRepository to
// leads.PendingReplyCounter so the leads inbox-list view can render
// the badge count without the leads context importing the inbox
// package directly.
type pendingReplyCounterAdapter struct {
	repo inbox.PendingReplyRepository
}

func newPendingReplyCounterAdapter(repo inbox.PendingReplyRepository) *pendingReplyCounterAdapter {
	return &pendingReplyCounterAdapter{repo: repo}
}

func (a *pendingReplyCounterAdapter) CountPendingByUser(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]int, error) {
	return a.repo.CountPendingByUser(ctx, userID)
}

// inboxEmailSenderAdapter bridges outbound.Sender to inbox.EmailSender
// so the email HITL dispatcher can dispatch approved replies through
// the same SMTP/Resend machinery used by the outbound sequence
// pipeline, without inbox having to import outbound directly.
//
// Idempotency caveat — empty key is passed to SendOneEmailFor. That
// covers the WITHIN-CALL retry loop in dispatchToResend (the loop
// reuses the same empty key across attempts, so a transient 5xx is
// safe to retry — Resend ignores the absent header consistently
// across attempts). It does NOT cover the BETWEEN-CALL case where
// an operator double-Approves: each Approve produces a fresh HTTP
// request with no Idempotency-Key, so Resend cannot dedup. The
// usecase optimistic-lock (status=pending) blocks the second
// Approve from running unless the first crashed pre-Update, which
// is a narrow race. Threading pr.ID.String() through the port is
// the obvious tightening when the race shows up in practice.
type inboxEmailSenderAdapter struct {
	sender *outbound.Sender
}

func newInboxEmailSenderAdapter(sender *outbound.Sender) *inboxEmailSenderAdapter {
	return &inboxEmailSenderAdapter{sender: sender}
}

func (a *inboxEmailSenderAdapter) SendEmail(ctx context.Context, userID uuid.UUID, to, subject, body string) error {
	return a.sender.SendOneEmailFor(ctx, userID, to, subject, body, "")
}

// Compile-time check that the adapter satisfies inbox.EmailSender so
// signature drift on the port breaks the build at the wiring edge.
var _ inbox.EmailSender = (*inboxEmailSenderAdapter)(nil)

// --- 1C EventApplier adapter (onec → leads/prospects boundary) ---
//
// Implements onec.EventApplier so the onec context never imports leads or
// prospects directly. Each handler resolves the Floq entity by the
// counterparty email extracted from the 1C payload. Actions that target an
// existing entity (payment/order/shipment) no-op when no lead matches or when
// the lead's state machine forbids the transition — surfaced as a log line, not
// an error, since a 1C webhook must not be failed for a benign mismatch.
// counterparty-created upserts a prospect.
//
// Dependencies are narrow interfaces (not concrete *leads/*prospects types) so
// the routing/transition logic is unit-testable with fakes.
type onecLeadLookup interface {
	GetLeadByEmailAddress(ctx context.Context, userID uuid.UUID, email string) (*leadsdomain.Lead, error)
}
type onecLeadMover interface {
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}
type onecProspectStore interface {
	FindByEmail(ctx context.Context, userID uuid.UUID, email string) (*prospectsdomain.Prospect, error)
	CreateProspect(ctx context.Context, input prospects.CreateProspectInput) (*prospectsdomain.Prospect, error)
}

type onecApplierAdapter struct {
	leadLookup onecLeadLookup
	leadMover  onecLeadMover
	prospects  onecProspectStore
	logger     *slog.Logger
}

func newOnecApplierAdapter(leadLookup onecLeadLookup, leadMover onecLeadMover, prospectStore onecProspectStore, logger *slog.Logger) *onecApplierAdapter {
	return &onecApplierAdapter{leadLookup: leadLookup, leadMover: leadMover, prospects: prospectStore, logger: logger}
}

// HandlePayment: counterparty paid → the deal is won.
func (a *onecApplierAdapter) HandlePayment(ctx context.Context, userID uuid.UUID, email string) error {
	return a.moveLeadByEmail(ctx, userID, email, leadsdomain.StatusWon)
}

// HandleOrderStatus: order moved → mark the lead as in active conversation.
func (a *onecApplierAdapter) HandleOrderStatus(ctx context.Context, userID uuid.UUID, email string) error {
	return a.moveLeadByEmail(ctx, userID, email, leadsdomain.StatusInConversation)
}

// HandleShipment: goods shipped → flag the lead for follow-up.
func (a *onecApplierAdapter) HandleShipment(ctx context.Context, userID uuid.UUID, email string) error {
	return a.moveLeadByEmail(ctx, userID, email, leadsdomain.StatusFollowup)
}

// moveLeadByEmail transitions the lead matched by email to target, skipping
// benignly when there is no match or the transition is illegal for the lead's
// current state. The legality decision belongs to the domain
// (Lead.TransitionTo), not this adapter: we attempt the move and swallow only
// the benign ErrInvalidTransition sentinel, surfaced as a log line — a 1C
// webhook must not be failed for a state mismatch. Any other error propagates.
func (a *onecApplierAdapter) moveLeadByEmail(ctx context.Context, userID uuid.UUID, email string, target leadsdomain.LeadStatus) error {
	if email == "" {
		return nil
	}
	lead, err := a.leadLookup.GetLeadByEmailAddress(ctx, userID, email)
	if err != nil {
		return err
	}
	if lead == nil {
		a.logger.Info("onec: no lead for counterparty email; action skipped", "target", target.String())
		return nil
	}
	if err := a.leadMover.UpdateStatus(ctx, lead.ID, target.String()); err != nil {
		// Benign outcomes for a 1C webhook: the lead's state machine forbids
		// this edge, or the lead vanished between the email lookup and the
		// update (a concurrent delete). Neither is a state the webhook should
		// fail on — skip with a log line. Anything else propagates.
		if errors.Is(err, leadsdomain.ErrInvalidTransition) || errors.Is(err, leadsdomain.ErrLeadNotFound) {
			a.logger.Info("onec: lead action skipped",
				"lead_id", lead.ID, "to", target.String(), "reason", err)
			return nil
		}
		return err
	}
	return nil
}

// HandleCounterpartyCreated upserts a prospect for a new 1C counterparty. An
// existing prospect (matched by email) is left untouched; a missing name falls
// back to the email so the prospect invariant (non-empty name) holds.
func (a *onecApplierAdapter) HandleCounterpartyCreated(ctx context.Context, userID uuid.UUID, email, name, company string) error {
	if email == "" {
		return nil
	}
	existing, err := a.prospects.FindByEmail(ctx, userID, email)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	if name == "" {
		name = email
	}
	_, err = a.prospects.CreateProspect(ctx, prospects.CreateProspectInput{
		UserID:  userID,
		Name:    name,
		Company: company,
		Email:   email,
	})
	return err
}

// Compile-time check that the adapter satisfies onec.EventApplier.
var _ onec.EventApplier = (*onecApplierAdapter)(nil)

// --- 1C qualification observer (leads → onec outbound boundary) ---
//
// Implements leadsdomain.QualificationObserver so the leads context can fire a
// post-qualification side-effect without importing onec. It translates a
// qualified Lead into a CounterpartyDraft and pushes it to 1C. All failures are
// swallowed (logged) — qualification must never fail because the 1C push did.
// counterpartyPusher is the narrow slice of the onec outbound use case this
// adapter needs — kept an interface (not the concrete *onec.OutboundUseCase) so
// the adapter's branching and goroutine can be unit-tested with a fake.
type counterpartyPusher interface {
	PushCounterparty(ctx context.Context, userID uuid.UUID, draft *onecdomain.CounterpartyDraft) error
}

type onecQualificationAdapter struct {
	outbound counterpartyPusher
	logger   *slog.Logger
}

func newOnecQualificationAdapter(outbound counterpartyPusher, logger *slog.Logger) *onecQualificationAdapter {
	return &onecQualificationAdapter{outbound: outbound, logger: logger}
}

// onecPushTimeout bounds the detached counterparty push (HTTP to 1C with
// retries) independently of the request that triggered qualification.
const onecPushTimeout = 35 * time.Second

// OnLeadQualified builds a counterparty draft from the lead and pushes it to 1C.
// A lead with neither name nor email cannot become a counterparty — skipped.
//
// The push runs in a detached goroutine on a fresh background context for two
// distinct reasons:
//
//  1. Latency/cancellation isolation (the reason for detaching from the REQUEST
//     context): a slow 1C must not block the user's /qualify call, and a client
//     disconnect must not cancel the push mid-flight — which would lose even the
//     'error' ledger entry the push records.
//  2. Not bound to the APP-lifecycle context: like the outbound email cron
//     goroutine, an in-flight push is not awaited on server shutdown, so a push
//     started inside the shutdown window can be lost. This is a deliberate
//     trade-off — outbound pushes lost to shutdown (or any gap) are exactly what
//     the scheduled reconciliation (#109) re-applies idempotently.
func (a *onecQualificationAdapter) OnLeadQualified(_ context.Context, lead *leadsdomain.Lead) {
	email := ""
	if lead.EmailAddress != nil {
		email = *lead.EmailAddress
	}
	draft, err := onecdomain.NewCounterpartyDraft(lead.ContactName, email, lead.Company)
	if err != nil {
		a.logger.Info("onec: qualified lead has no name/email; counterparty push skipped", "lead_id", lead.ID)
		return
	}
	userID, leadID := lead.UserID, lead.ID
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), onecPushTimeout)
		defer cancel()
		if err := a.outbound.PushCounterparty(ctx, userID, draft); err != nil {
			a.logger.Warn("onec: counterparty push to 1C failed", "lead_id", leadID, "err", err)
		}
	}()
}

// Compile-time check that the adapter satisfies the leads observer port.
var _ leadsdomain.QualificationObserver = (*onecQualificationAdapter)(nil)

// queueDepthAdapter bridges the inbox pending-reply repository to the
// metrics package's QueueDepthSource port, so the metrics context polls
// queue depth without importing inbox (cross-context wiring lives here,
// at the composition root).
type queueDepthAdapter struct {
	repo *inbox.PendingReplyRepo
}

func (a queueDepthAdapter) QueueDepths(ctx context.Context) (map[string]int, error) {
	return a.repo.CountPendingByKind(ctx)
}
