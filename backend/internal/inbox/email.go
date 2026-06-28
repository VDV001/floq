package inbox

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"log/slog"
	"strconv"
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
	store                 ConfigStore
	repo                  LeadRepository
	prospectRepo          ProspectRepository
	seqRepo               SequenceRepository
	qualJobs              QualificationJobEnqueuer
	analyzer              *attachments.Analyzer
	identityLinker        IdentityLinker
	enricher              EnrichmentEnqueuer
	pendingProposer       PendingReplyProposer
	leadCreatedEmitter LeadCreatedEmitter
	tx                 TxManager
	retries            *retryTracker
	onQuarantine       func(channel string)
	bookingLink          string
	logger               *slog.Logger
	ownerID               uuid.UUID
	dialer                proxy.ContextDialer

	fallbackHost     string
	fallbackPort     string
	fallbackUser     string
	fallbackPassword string
}

func NewEmailPoller(store ConfigStore, ownerID uuid.UUID, fallbackHost, fallbackPort, fallbackUser, fallbackPassword string, repo LeadRepository, prospectRepo ProspectRepository, seqRepo SequenceRepository, dialer proxy.ContextDialer, opts ...EmailPollerOption) *EmailPoller {
	p := &EmailPoller{
		store:            store,
		repo:             repo,
		prospectRepo:     prospectRepo,
		seqRepo:          seqRepo,
		ownerID:          ownerID,
		dialer:           dialer,
		logger:           slog.Default(),
		retries:          newRetryTracker(defaultIntakeMaxAttempts),
		onQuarantine:     func(string) {},
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

// SetLeadCreatedEmitter wires the best-effort post-commit lead.created emitter
// (#199 / #206). The source email is already marked \Seen by the time a lead is
// written, so intake cannot be fail-closed without losing the lead — the event
// is emitted post-commit, at-most-once, NOT inside a transaction.
func (e *EmailPoller) SetLeadCreatedEmitter(em LeadCreatedEmitter) { e.leadCreatedEmitter = em }

// SetTxManager wires the transaction manager that makes new-lead intake
// fail-closed (#206): the lead row and the lead.created enqueue commit together
// or roll back together. Safe for the email poller because \Seen is marked only
// after a successful intake, so a rollback leaves the source email unseen and the
// next poll re-processes it.
func (e *EmailPoller) SetTxManager(tx TxManager) { e.tx = tx }

// SetIntakeRetryCap overrides the number of consecutive failed intake attempts
// (per source email UID) tolerated before a poison email is quarantined. A
// non-positive cap disables quarantine (fail-closed forever — the pre-#208
// behaviour). Wired from config at the composition root (#208).
func (e *EmailPoller) SetIntakeRetryCap(maxAttempts int) {
	e.retries = newRetryTracker(maxAttempts)
}

// SetQuarantineObserver wires the callback fired once when an email is
// quarantined (retry cap reached), so the composition root can publish a metric
// without this package importing the metrics package (#208).
func (e *EmailPoller) SetQuarantineObserver(fn func(channel string)) {
	if fn != nil {
		e.onQuarantine = fn
	}
}

// reconcileIntake records a processEmail outcome and reports whether the source
// email should be marked \Seen (consumed). A success consumes immediately and
// resets the failure count. A transient error leaves the email unseen for the
// next poll to retry (#206 fail-closed) — until the retry cap is reached, at
// which point the email is quarantined: consumed so it stops hot-looping, and
// reported via the quarantine observer for alerting. The email itself stays in
// the mailbox (only the \Seen flag is set) for manual recovery (#208).
func (e *EmailPoller) reconcileIntake(key, fromEmail string, procErr error) (markSeen bool) {
	if procErr == nil {
		e.retries.succeed(key)
		return true
	}
	attempts, exhausted := e.retries.fail(key)
	if !exhausted {
		log.Printf("[email-poller] intake failed for %s (attempt %d), will retry next poll: %v", fromEmail, attempts, procErr)
		return false
	}
	log.Printf("[email-poller] intake quarantined for %s after %d attempts, marking seen: %v", fromEmail, attempts, procErr)
	e.onQuarantine("email")
	return true
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
			key := strconv.FormatUint(uint64(buf.UID), 10)
			if e.reconcileIntake(key, fromEmail, e.processEmail(ctx, fromName, fromEmail, bodyText, atts)) {
				processedUIDs = append(processedUIDs, buf.UID)
			}
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

// processEmail handles one inbound email. It returns a non-nil error only for
// transient failures where the caller must leave the source email unseen so the
// next poll retries it (#206 fail-closed). Permanent/skip conditions (invalid
// entity, swallowed best-effort side-effects) return nil so the email is marked
// \Seen and not re-processed forever.
func (e *EmailPoller) processEmail(ctx context.Context, fromName, fromEmail, body string, atts []attachments.Attachment) error {
	fromEmail = normalize.Email(fromEmail)
	existing, err := e.repo.GetLeadByEmailAddress(ctx, e.ownerID, fromEmail)
	if err != nil {
		// Transient: signal retry rather than dropping the email (#206).
		return fmt.Errorf("look up lead for %s: %w", fromEmail, err)
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
			return nil
		}
		lead = newLead
		if hasProspectMatch {
			lead.SourceID = prospect.SourceID
		}
		// Build the durable auto-qualification job (the AI qualifier sees the
		// email body plus any extracted attachment text). It is enqueued
		// atomically with the lead below so a retry never loses the
		// qualification (#206 Part C); a worker scores it asynchronously and
		// emits lead.qualified transactionally.
		job := e.newQualificationJob(ctx, lead, fromName, body, atts)
		// #206: persist the lead, enqueue lead.created, and enqueue the
		// qualification job atomically. A failure rolls all of them back and the
		// error propagates so the poll loop leaves the email unseen for retry
		// (fail-closed). Safe because \Seen is marked only after a nil return.
		if err := e.commitLeadIntake(ctx, lead, job); err != nil {
			log.Printf("[email-poller] error creating lead for %s, will retry next poll: %v", fromEmail, err)
			return err
		}
		log.Printf("[email-poller] new lead created for %s (%s)", fromEmail, contactName)

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
		return nil
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

	return nil
}

// newQualificationJob builds the durable auto-qualification job for a new email
// lead, capturing the qualifier input (the body plus any extracted attachment
// text) at enqueue time. Returns nil when no enqueuer is wired or the input is
// empty — qualification is simply skipped, never blocking intake.
func (e *EmailPoller) newQualificationJob(ctx context.Context, lead *InboxLead, contactName, body string, atts []attachments.Attachment) *QualificationJob {
	if e.qualJobs == nil {
		return nil
	}
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
	job, err := NewQualificationJob(lead.ID, lead.UserID, contactName, lead.Channel, qualifyText)
	if err != nil {
		e.logger.WarnContext(ctx, "inbox: skip qualification job", "lead", lead.ID, "err", err)
		return nil
	}
	return job
}

// SetQualificationEnqueuer wires the durable qualification queue. When set, every
// new lead enqueues a job (atomically with the lead) for the qualification worker
// to score. When nil, qualification is skipped (e.g. tests).
func (e *EmailPoller) SetQualificationEnqueuer(q QualificationJobEnqueuer) { e.qualJobs = q }

// commitLeadIntake persists a new inbound lead and, atomically with it, enqueues
// its lead.created event and the auto-qualification job. When a transaction
// manager is wired (production), all three run inside one WithTx so they commit
// or roll back together — a failed enqueue undoes the lead and returns an error
// (fail-closed, #206), and the caller leaves the email unseen for retry.
// Without a tx (tests), it falls back to a plain create with best-effort
// post-commit side-effects. job may be nil (qualification disabled).
func (e *EmailPoller) commitLeadIntake(ctx context.Context, lead *InboxLead, job *QualificationJob) error {
	if e.tx != nil && (e.leadCreatedEmitter != nil || job != nil) {
		return e.tx.WithTx(ctx, func(txCtx context.Context) error {
			if err := e.repo.CreateLead(txCtx, lead); err != nil {
				return err
			}
			if e.leadCreatedEmitter != nil {
				if err := e.leadCreatedEmitter.EmitLeadCreated(txCtx, lead); err != nil {
					return err
				}
			}
			if job != nil {
				if err := e.qualJobs.EnqueueQualificationJob(txCtx, job); err != nil {
					return err
				}
			}
			return nil
		})
	}
	if err := e.repo.CreateLead(ctx, lead); err != nil {
		return err
	}
	if e.leadCreatedEmitter != nil {
		if err := e.leadCreatedEmitter.EmitLeadCreated(ctx, lead); err != nil {
			log.Printf("[email-poller] lead.created emit failed (best-effort): %v", err)
		}
	}
	if job != nil {
		if err := e.qualJobs.EnqueueQualificationJob(ctx, job); err != nil {
			log.Printf("[email-poller] qualification enqueue failed (best-effort): %v", err)
		}
	}
	return nil
}
