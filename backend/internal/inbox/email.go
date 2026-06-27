package inbox

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"log/slog"
	"strings"
	"time"

	auditdomain "github.com/daniil/floq/internal/audit/domain"
	"github.com/daniil/floq/internal/inbox/attachments"
	"github.com/daniil/floq/internal/normalize"
	"github.com/daniil/floq/internal/proxy"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
	"github.com/google/uuid"
)

// EmailPoller polls an IMAP mailbox for new emails and creates leads.
// When analyzer is non-nil, attachments on each inbound message are
// extracted and their text content is appended to the qualification
// context. A nil analyzer keeps the legacy text-only behaviour.
type EmailPoller struct {
	store           ConfigStore
	repo            LeadRepository
	prospectRepo    ProspectRepository
	seqRepo         SequenceRepository
	aiClient        AIQualifier
	analyzer        *attachments.Analyzer
	identityLinker  IdentityLinker
	enricher        EnrichmentEnqueuer
	pendingProposer PendingReplyProposer
	leadCreatedObserver LeadCreatedObserver
	bookingLink     string
	logger          *slog.Logger
	ownerID         uuid.UUID
	dialer          proxy.ContextDialer

	fallbackHost     string
	fallbackPort     string
	fallbackUser     string
	fallbackPassword string
}

func NewEmailPoller(store ConfigStore, ownerID uuid.UUID, fallbackHost, fallbackPort, fallbackUser, fallbackPassword string, repo LeadRepository, prospectRepo ProspectRepository, seqRepo SequenceRepository, aiClient AIQualifier, dialer proxy.ContextDialer, opts ...EmailPollerOption) *EmailPoller {
	p := &EmailPoller{
		store:            store,
		repo:             repo,
		prospectRepo:     prospectRepo,
		seqRepo:          seqRepo,
		aiClient:         aiClient,
		ownerID:          ownerID,
		dialer:           dialer,
		logger:           slog.Default(),
		fallbackHost:     fallbackHost,
		fallbackPort:     fallbackPort,
		fallbackUser:     fallbackUser,
		fallbackPassword: fallbackPassword,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// EmailPollerOption configures optional EmailPoller behaviour. The
// existing call sites in cmd/server keep working unchanged; new
// capabilities (currently only the attachments analyzer) plug in
// through variadic options.
type EmailPollerOption func(*EmailPoller)

// WithAttachmentAnalyzer wires the analyzer used to extract text from
// PDFs and screenshots so it reaches the AI qualification step. Pass
// nil to disable; the poller silently degrades to text-only.
func WithAttachmentAnalyzer(a *attachments.Analyzer) EmailPollerOption {
	return func(p *EmailPoller) { p.analyzer = a }
}

// WithIdentityLinker wires the IdentityLinker used to resolve and
// link each newly created lead to a unified Identity. Pass nil (or
// omit the option) to disable; the poller continues to create leads
// untouched. Linker errors are logged and swallowed — the inbound
// flow must never block on identity-aggregation backend hiccups.
func WithIdentityLinker(l IdentityLinker) EmailPollerOption {
	return func(p *EmailPoller) { p.identityLinker = l }
}

// WithEmailEnricher wires the cross-context enrichment enqueuer so each newly
// created lead's company domain is queued for background scraping. Best-effort:
// omit it (or pass nil) to disable. Errors are logged, never block inbound.
func WithEmailEnricher(e EnrichmentEnqueuer) EmailPollerOption {
	return func(p *EmailPoller) { p.enricher = e }
}

// WithLogger overrides the default slog.Logger so the email poller
// emits structured warnings to the same handler as the rest of the
// server. Pass nil to keep slog.Default().
func WithLogger(l *slog.Logger) EmailPollerOption {
	return func(p *EmailPoller) {
		if l != nil {
			p.logger = l
		}
	}
}

// WithEmailPendingReplyProposer wires the HITL queue for the
// email-channel auto-draft branch. Symmetric with the Telegram bot's
// proposer wiring: when DetectCallAgreement matches the email body,
// the poller enqueues a booking-link pending reply for operator
// approval instead of sending the URL directly. When nil (or
// omitted), the booking-link branch is suppressed entirely.
//
// Named asymmetrically to its telegram-side sibling because Go does
// not allow option-name collisions across different option types in
// the same package.
func WithEmailPendingReplyProposer(p PendingReplyProposer) EmailPollerOption {
	return func(e *EmailPoller) { e.pendingProposer = p }
}

// WithEmailBookingLink sets the calendar URL interpolated into the
// auto-drafted booking-link reply. Must be set when
// WithEmailPendingReplyProposer is also wired; otherwise the
// formatted reply would carry an empty URL.
func WithEmailBookingLink(link string) EmailPollerOption {
	return func(e *EmailPoller) { e.bookingLink = link }
}

// SetPendingProposer wires the HITL queue after construction. Used by
// the composition root to break a wiring cycle when the inbox usecase
// (acting as proposer) depends on transports built from the poller's
// own collaborators.
func (e *EmailPoller) SetPendingProposer(p PendingReplyProposer) {
	e.pendingProposer = p
}

// SetLeadCreatedObserver wires the post-lead-creation hook after construction
// (the webhooks usecase it bridges to is built later in the composition root).
func (e *EmailPoller) SetLeadCreatedObserver(o LeadCreatedObserver) {
	e.leadCreatedObserver = o
}

func (e *EmailPoller) Start(ctx context.Context) {
	log.Println("email poller started (every 60s)")

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	e.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("email poller shutting down")
			return
		case <-ticker.C:
			e.poll(ctx)
		}
	}
}

func (e *EmailPoller) resolveConfig(ctx context.Context) (host, port, user, password string) {
	host, port, user, password = e.fallbackHost, e.fallbackPort, e.fallbackUser, e.fallbackPassword
	if cfg, err := e.store.GetConfig(ctx, e.ownerID); err == nil {
		host = ResolveConfig(cfg.IMAPHost, host)
		port = ResolveConfig(cfg.IMAPPort, port)
		user = ResolveConfig(cfg.IMAPUser, user)
		password = ResolveConfig(cfg.IMAPPassword, password)
	}
	return
}

func (e *EmailPoller) poll(ctx context.Context) {
	host, port, user, password := e.resolveConfig(ctx)
	if host == "" || user == "" || password == "" {
		return
	}

	addr := host + ":" + port
	var c *imapclient.Client
	var err error

	if e.dialer != nil {
		rawConn, dialErr := e.dialer.DialContext(ctx, "tcp", addr)
		if dialErr != nil {
			log.Printf("[email-poller] proxy dial error: %v", dialErr)
			return
		}
		tlsConn := tls.Client(rawConn, &tls.Config{ServerName: host})
		c = imapclient.New(tlsConn, nil)
	} else {
		c, err = imapclient.DialTLS(addr, nil)
		if err != nil {
			log.Printf("[email-poller] connect error: %v", err)
			return
		}
	}
	defer c.Close()

	if err := c.Login(user, password).Wait(); err != nil {
		log.Printf("[email-poller] login error: %v", err)
		return
	}

	if _, err := c.Select("INBOX", nil).Wait(); err != nil {
		log.Printf("[email-poller] select INBOX error: %v", err)
		return
	}

	// Search unseen messages
	criteria := &imap.SearchCriteria{
		NotFlag: []imap.Flag{imap.FlagSeen},
	}
	searchData, err := c.UIDSearch(criteria, nil).Wait()
	if err != nil {
		log.Printf("[email-poller] search error: %v", err)
		_ = c.Logout().Wait()
		return
	}

	uids := searchData.AllUIDs()
	if len(uids) == 0 {
		_ = c.Logout().Wait()
		return
	}

	log.Printf("[email-poller] found %d unseen emails", len(uids))

	// Fetch messages with envelope + body
	uidSet := imap.UIDSetNum(uids...)
	bodySection := &imap.FetchItemBodySection{}
	fetchOptions := &imap.FetchOptions{
		Envelope:    true,
		UID:         true,
		BodySection: []*imap.FetchItemBodySection{bodySection},
	}

	fetchCmd := c.Fetch(uidSet, fetchOptions)

	var processedUIDs []imap.UID

	for {
		msgData := fetchCmd.Next()
		if msgData == nil {
			break
		}

		// Collect into buffer for easy access
		buf, err := msgData.Collect()
		if err != nil {
			log.Printf("[email-poller] collect error: %v", err)
			continue
		}

		fromName := ""
		fromEmail := ""
		if buf.Envelope != nil && len(buf.Envelope.From) > 0 {
			from := buf.Envelope.From[0]
			fromName = from.Name
			fromEmail = from.Addr()
			if fromName == "" {
				fromName = fromEmail
			}
		}

		// Skip emails from ourselves or automated/noreply senders
		if fromEmail == user || shouldSkipEmail(fromEmail) {
			processedUIDs = append(processedUIDs, buf.UID)
			continue
		}

		// Extract text body + any non-inline attachments. Both share the
		// same raw body bytes, so we parse the MIME structure twice for
		// clarity rather than threading a more complex helper through
		// the existing test surface.
		var bodyText string
		var atts []attachments.Attachment
		if len(buf.BodySection) > 0 {
			bodyText = extractTextBody(buf.BodySection[0].Bytes)
			atts = extractAttachments(buf.BodySection[0].Bytes)
		}

		if fromEmail != "" && bodyText != "" {
			e.processEmail(ctx, fromName, fromEmail, bodyText, atts)
			processedUIDs = append(processedUIDs, buf.UID)
		}
	}

	if err := fetchCmd.Close(); err != nil {
		log.Printf("[email-poller] fetch error: %v", err)
	}

	// Mark processed messages as seen
	if len(processedUIDs) > 0 {
		markSet := imap.UIDSetNum(processedUIDs...)
		storeFlags := &imap.StoreFlags{
			Op:     imap.StoreFlagsAdd,
			Flags:  []imap.Flag{imap.FlagSeen},
			Silent: true,
		}
		if err := c.Store(markSet, storeFlags, nil).Close(); err != nil {
			log.Printf("[email-poller] mark seen error: %v", err)
		}
	}

	_ = c.Logout().Wait()
}

// shouldSkipEmail returns true for automated/noreply/service addresses.
func shouldSkipEmail(email string) bool {
	email = strings.ToLower(email)
	prefixes := []string{"noreply@", "no-reply@", "no_reply@", "mailer-daemon@", "postmaster@"}
	for _, p := range prefixes {
		if strings.HasPrefix(email, p) {
			return true
		}
	}
	// Skip common service domains
	domains := []string{"@yandex.ru", "@yandex.com", "@google.com", "@gmail.com", "@mail.ru",
		"@facebook.com", "@facebookmail.com", "@linkedin.com", "@twitter.com",
		"@github.com", "@apple.com", "@microsoft.com"}
	for _, d := range domains {
		if strings.HasSuffix(email, d) {
			// Only skip if it looks automated (contains service-like prefixes)
			localPart := strings.Split(email, "@")[0]
			for _, sp := range []string{"noreply", "no-reply", "no_reply", "mailer-daemon", "postmaster", "bounce", "notification", "notify", "newsletter", "updates"} {
				if strings.Contains(localPart, sp) {
					return true
				}
			}
		}
	}
	return false
}

// extractAttachments walks the multipart MIME tree of raw and returns
// every non-inline part — i.e. real attachments — as attachments.Attachment
// records the analyser can consume. Inline body parts (text/plain,
// text/html) are handled by extractTextBody and skipped here. A
// malformed or non-MIME body yields an empty slice rather than an
// error; the caller treats no-attachments and parse-failed identically.
func extractAttachments(raw []byte) []attachments.Attachment {
	mr, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	var atts []attachments.Attachment
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		ah, ok := p.Header.(*mail.AttachmentHeader)
		if !ok {
			continue
		}
		data, err := io.ReadAll(p.Body)
		if err != nil {
			continue
		}
		filename, _ := ah.Filename()
		contentType, _, _ := ah.ContentType()
		atts = append(atts, attachments.Attachment{
			Filename:    filename,
			ContentType: contentType,
			Data:        data,
		})
	}
	return atts
}

func extractTextBody(raw []byte) string {
	mr, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		// Fallback: treat as plain text
		return strings.TrimSpace(string(raw))
	}

	var textBody string
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		if _, ok := p.Header.(*mail.InlineHeader); ok {
			ct := p.Header.Get("Content-Type")
			b, err := io.ReadAll(p.Body)
			if err != nil {
				continue
			}
			text := strings.TrimSpace(string(b))
			if strings.Contains(ct, "text/plain") || textBody == "" {
				textBody = text
			}
		}
	}
	return textBody
}

func (e *EmailPoller) processEmail(ctx context.Context, fromName, fromEmail, body string, atts []attachments.Attachment) {
	fromEmail = normalize.Email(fromEmail)
	existing, err := e.repo.GetLeadByEmailAddress(ctx, e.ownerID, fromEmail)
	if err != nil {
		log.Printf("[email-poller] error looking up lead for %s: %v", fromEmail, err)
		return
	}

	isNewLead := existing == nil
	var lead *InboxLead

	// Check if sender is a known prospect
	prospect, prospectErr := e.prospectRepo.FindByEmail(ctx, e.ownerID, fromEmail)
	hasProspectMatch := prospectErr == nil && prospect != nil && prospect.Status != ProspectStatusConverted

	if isNewLead {
		contactName := fromName
		company := ""
		if hasProspectMatch {
			if contactName == fromEmail && prospect.Name != "" {
				contactName = prospect.Name
			}
			if prospect.Company != "" {
				company = prospect.Company
			}
		}

		emailAddr := fromEmail
		newLead, err := NewInboxLead(e.ownerID, ChannelEmail, contactName, company, body, nil, &emailAddr)
		if err != nil {
			log.Printf("[email-poller] error creating lead entity: %v", err)
			return
		}
		lead = newLead
		if hasProspectMatch {
			lead.SourceID = prospect.SourceID
		}
		if err := e.repo.CreateLead(ctx, lead); err != nil {
			log.Printf("[email-poller] error creating lead: %v", err)
			return
		}
		log.Printf("[email-poller] new lead created for %s (%s)", fromEmail, contactName)
		if e.leadCreatedObserver != nil {
			e.leadCreatedObserver.OnLeadCreated(ctx, lead)
		}

		if e.identityLinker != nil {
			if err := e.identityLinker.LinkLeadToIdentity(ctx, e.ownerID, lead.ID, fromEmail, "", ""); err != nil {
				e.logger.WarnContext(ctx, "inbox: identity link failed",
					"lead", lead.ID, "channel", "email", "err", err)
			}
		}

		// Best-effort: queue background company-data enrichment by the sender's
		// email domain. Failures are logged, never block the inbound flow.
		if e.enricher != nil {
			if err := e.enricher.Enqueue(ctx, e.ownerID, fromEmail); err != nil {
				e.logger.WarnContext(ctx, "inbox: enrichment enqueue failed",
					"lead", lead.ID, "err", err)
			}
		}

		// Auto-convert matched prospect to lead
		if hasProspectMatch {
			if convErr := e.prospectRepo.ConvertToLead(ctx, prospect.ID, lead.ID); convErr != nil {
				log.Printf("[email-poller] error converting prospect %s: %v", prospect.ID, convErr)
			} else {
				log.Printf("[email-poller] prospect %s auto-converted to lead %s", prospect.ID, lead.ID)
			}
			// Mark outbound messages as replied
			_ = e.seqRepo.MarkRepliedByProspect(ctx, prospect.ID)
		}
	} else {
		lead = existing
		// Re-engagement: a new inbound email on an archived lead resurfaces it
		// so the reply reappears in the inbox feed instead of attaching to a
		// hidden lead and being silently lost (symmetric with telegram.go).
		if existing.ArchivedAt != nil {
			if err := e.repo.UnarchiveLead(ctx, lead.ID); err != nil {
				log.Printf("[email-poller] error unarchiving lead %s on re-engagement: %v", lead.ID, err)
			} else {
				lead.ArchivedAt = nil
			}
		}
	}

	message := NewInboxMessage(lead.ID, DirectionInbound, body)
	if err := e.repo.CreateMessage(ctx, message); err != nil {
		log.Printf("[email-poller] error creating message: %v", err)
		return
	}

	// HITL gate for the email booking-link branch — symmetric with the
	// Telegram bot (telegram.go). When DetectCallAgreement triggers,
	// enqueue a pending reply for operator approval rather than
	// sending the calendar URL directly via SMTP/Resend. The secure
	// default when no proposer is wired is to suppress the branch
	// entirely; we do NOT fall back to instant send.
	if DetectCallAgreement(body) {
		switch {
		case e.pendingProposer == nil:
			e.logger.WarnContext(ctx, "email booking link suppressed: no pending reply proposer wired",
				"lead_id", lead.ID.String(), "email", fromEmail)
		case e.bookingLink == "":
			// Enqueueing with an empty URL would let an operator
			// approve a customer-visible message that says "here is
			// your calendar link: " followed by nothing. Suppress the
			// branch instead — operator can write the message
			// manually if needed.
			e.logger.WarnContext(ctx, "email booking link suppressed: bookingLink not configured",
				"lead_id", lead.ID.String(), "email", fromEmail)
		default:
			bookingMsg := fmt.Sprintf(bookingLinkReplyTemplate, e.bookingLink)
			if _, err := e.pendingProposer.Propose(ctx, lead.UserID, lead.ID, ChannelEmail, PendingReplyKindBookingLink, bookingMsg, body); err != nil {
				e.logger.WarnContext(ctx, "failed to enqueue email booking reply for approval",
					"lead_id", lead.ID.String(), "error", err)
			}
		}
	}

	if isNewLead {
		// Build the text the AI qualifier sees: the email body plus any
		// extracted attachment content. We don't mutate lead.FirstMessage
		// (that's the conversation record) — qualifyText is ephemeral.
		qualifyText := body
		if e.analyzer != nil {
			imgLeadID := lead.ID
			imgCtx := auditdomain.ContextWithCallMeta(ctx, auditdomain.CallMeta{
				UserID:      lead.UserID,
				LeadID:      &imgLeadID,
				RequestType: auditdomain.RequestTypeImageAnalysis,
			})
			for _, att := range atts {
				res := e.analyzer.Analyze(imgCtx, att)
				if res.Skipped != "" {
					e.logger.WarnContext(ctx, "inbox: attachment skipped",
						"filename", att.Filename, "reason", res.Skipped, "err", res.Err)
					continue
				}
				qualifyText += "\n\n[Вложение: " + att.Filename + "]\n" + res.Text
			}
		}

		go func() {
			qCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			qLeadID := lead.ID
			qCtx = auditdomain.ContextWithCallMeta(qCtx, auditdomain.CallMeta{
				UserID:      lead.UserID,
				LeadID:      &qLeadID,
				RequestType: auditdomain.RequestTypeQualification,
			})
			result, err := e.aiClient.Qualify(qCtx, fromName, string(lead.Channel), qualifyText)
			if err != nil {
				log.Printf("[email-poller] qualification error for lead %s: %v", lead.ID, err)
				return
			}

			q := &InboxQualification{
				ID:                uuid.New(),
				LeadID:            lead.ID,
				IdentifiedNeed:    result.IdentifiedNeed,
				EstimatedBudget:   result.EstimatedBudget,
				Deadline:          result.Deadline,
				Score:             result.Score,
				ScoreReason:       result.ScoreReason,
				RecommendedAction: result.RecommendedAction,
				ProviderUsed:      e.aiClient.ProviderName(),
				GeneratedAt:       time.Now().UTC(),
			}
			if err := e.repo.UpsertQualification(qCtx, q); err != nil {
				log.Printf("[email-poller] error saving qualification for lead %s: %v", lead.ID, err)
				return
			}
			if err := e.repo.UpdateLeadStatus(qCtx, lead.ID, StatusQualified); err != nil {
				log.Printf("[email-poller] error updating lead status for %s: %v", lead.ID, err)
				return
			}
			log.Printf("[email-poller] lead %s qualified (score=%d)", lead.ID, result.Score)
		}()
	}
}
