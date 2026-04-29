package inbox

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
	"github.com/google/uuid"
)

// ContextDialer allows dialing TCP connections through a proxy.
type ContextDialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// EmailPoller polls an IMAP mailbox for new emails and creates leads.
type EmailPoller struct {
	store        ConfigStore
	repo         LeadRepository
	prospectRepo ProspectRepository
	seqRepo      SequenceRepository
	aiClient     AIQualifier
	ownerID      uuid.UUID
	dialer       ContextDialer

	fallbackHost     string
	fallbackPort     string
	fallbackUser     string
	fallbackPassword string
}

func NewEmailPoller(store ConfigStore, ownerID uuid.UUID, fallbackHost, fallbackPort, fallbackUser, fallbackPassword string, repo LeadRepository, prospectRepo ProspectRepository, seqRepo SequenceRepository, aiClient AIQualifier, dialer ContextDialer) *EmailPoller {
	return &EmailPoller{
		store:            store,
		repo:             repo,
		prospectRepo:     prospectRepo,
		seqRepo:          seqRepo,
		aiClient:         aiClient,
		ownerID:          ownerID,
		dialer:           dialer,
		fallbackHost:     fallbackHost,
		fallbackPort:     fallbackPort,
		fallbackUser:     fallbackUser,
		fallbackPassword: fallbackPassword,
	}
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

		// Extract text body from body section
		var bodyText string
		if len(buf.BodySection) > 0 {
			bodyText = extractTextBody(buf.BodySection[0].Bytes)
		}

		if fromEmail != "" && bodyText != "" {
			e.processEmail(ctx, fromName, fromEmail, bodyText)
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

func (e *EmailPoller) processEmail(ctx context.Context, fromName, fromEmail, body string) {
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
	}

	message := NewInboxMessage(lead.ID, DirectionInbound, body)
	if err := e.repo.CreateMessage(ctx, message); err != nil {
		log.Printf("[email-poller] error creating message: %v", err)
		return
	}

	if isNewLead {
		go func() {
			qCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			result, err := e.aiClient.Qualify(qCtx, fromName, string(lead.Channel), lead.FirstMessage)
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
