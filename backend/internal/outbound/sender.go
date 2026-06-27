package outbound

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
	"net/smtp"
	"sort"
	"strings"
	"sync"
	"time"

	prospectsdomain "github.com/daniil/floq/internal/prospects/domain"
	"github.com/daniil/floq/internal/proxy"
	seqdomain "github.com/daniil/floq/internal/sequences/domain"
	settingsdomain "github.com/daniil/floq/internal/settings/domain"
	"github.com/google/uuid"
)

// consentReason is the lawful-basis recorded for a cold (none-consent)
// outbound send. A message only reaches the send queue after operator HITL
// approval of the draft, which is the legitimate-interest basis for the
// contact. Logged per cold send (see authorizeConsent) so the justification
// is auditable rather than a silent bypass.
const consentReason = "operator-approved outbound sequence send"

// authorizeConsent applies the domain consent gate before a send. Withdrawal
// is honored absolutely; a cold 'none' prospect is let through under a logged
// lawful-basis override; an 'obtained' prospect sends freely. Returns false
// (and logs why) when the send must be skipped.
//
// Suppression-list checks land ahead of this in slice 1.2 (see ADR-002); for
// now the gate is the AuthorizeOutbound rule alone.
func authorizeConsent(prospect *prospectsdomain.Prospect, msgID uuid.UUID) bool {
	override, err := prospectsdomain.NewOutboundOverride(consentReason)
	if err != nil {
		// Unreachable: consentReason is a non-empty constant. Skip defensively.
		log.Printf("[outbound][consent] invalid override for msg %s: %v", msgID, err)
		return false
	}
	if err := prospect.AuthorizeOutbound(&override); err != nil {
		log.Printf("[outbound][consent] skipping msg %s to prospect %s: %v", msgID, prospect.ID, err)
		return false
	}
	if prospect.Consent.Status != prospectsdomain.ConsentStatusObtained {
		log.Printf("[outbound][consent] cold send msg %s to prospect %s (consent=%q) under lawful-basis override: %s",
			msgID, prospect.ID, prospect.Consent.Status, consentReason)
	}
	return true
}

// isSuppressed reports whether a send to address on channel must be skipped:
// either the address is on the suppression list, or the check itself failed.
// It fails CLOSED — an unverifiable suppression state must never result in a
// send, since suppression is the hard compliance pre-check ahead of consent.
// An empty address has nothing to match and is not suppressed.
func (s *Sender) isSuppressed(ctx context.Context, userID uuid.UUID, channel prospectsdomain.SuppressionChannel, address string, msgID uuid.UUID) bool {
	if address == "" {
		return false
	}
	suppressed, err := s.prospectRepo.IsSuppressed(ctx, userID, channel, address)
	if err != nil {
		log.Printf("[outbound][suppression] check failed for msg %s, skipping (fail-closed): %v", msgID, err)
		return true
	}
	if suppressed {
		log.Printf("[outbound][suppression] skipping msg %s: address suppressed on %s", msgID, channel)
		return true
	}
	return false
}

type Sender struct {
	store        ConfigStore
	ownerID      uuid.UUID
	fallbackKey  string
	fromAddress  string
	appBaseURL   string
	smtpHost     string
	smtpPort     string
	smtpUser     string
	smtpPassword string
	seqRepo      OutboundRepository
	prospectRepo ProspectLookup
	tgRepo       TelegramSessionStore
	tgMessenger  TelegramMessenger
	dialer       proxy.ContextDialer
	httpClient   *http.Client

	// unsubscribeSecret signs the per-email unsubscribe tokens. Set via
	// SetUnsubscribeSecret at wiring time; when empty, emails carry no
	// unsubscribe link or List-Unsubscribe header.
	unsubscribeSecret string

	// guard is the layer-3 outbound send validator. Set via SetSendGuard at
	// wiring time; nil means no extra validation (the consent + suppression
	// gates still apply). Production always wires it.
	guard SendGuard

	// tx + seqEmitter drive the #199 transactional outbox: when both are set,
	// the dispatch's sent/bounced mark and the sequence.completed enqueue commit
	// in one transaction. nil (webhooks disabled) marks the dispatch only, with
	// no completion event.
	tx         TxManager
	seqEmitter SequenceCompletionEmitter

	tgLastSent time.Time
	tgRateMu   sync.Mutex
}

// SetUnsubscribeSecret configures the HMAC secret used to mint per-email
// unsubscribe links. Called once at composition time (mirrors the optional-
// dependency setter pattern used elsewhere in wiring). When unset, outbound
// emails omit the unsubscribe link and List-Unsubscribe headers.
func (s *Sender) SetUnsubscribeSecret(secret string) { s.unsubscribeSecret = secret }

// SetSendGuard injects the layer-3 outbound send guard.
func (s *Sender) SetSendGuard(guard SendGuard) { s.guard = guard }

// SetTxManager wires the transaction manager (#199) so the dispatch mark and the
// sequence.completed enqueue commit together.
func (s *Sender) SetTxManager(tx TxManager) { s.tx = tx }

// SetSequenceCompletionEmitter wires the in-transaction sequence.completed outbox
// emitter (#199). With no emitter (or TxManager) a dispatch is marked with no
// completion event — the webhooks-disabled path.
func (s *Sender) SetSequenceCompletionEmitter(e SequenceCompletionEmitter) { s.seqEmitter = e }

// commitDispatch persists a dispatch outcome (markFn = MarkSent or MarkBounced).
// When a transactional emitter is wired (#199), the mark and the
// sequence.completed enqueue run in ONE db.WithTx so they commit together
// (fail-closed). A rollback leaves the message un-marked and it is re-sent on the
// next tick — safe on the idempotent Resend path, and within the pre-existing
// accepted SMTP duplicate window (see sendViaSMTPWith). With no emitter/TxManager
// (webhooks disabled) it persists the mark only, with no completion event.
// Returns the mark/emit error so the caller logs and skips this message's
// remaining post-send steps.
func (s *Sender) commitDispatch(ctx context.Context, msg seqdomain.OutboundMessage, userID uuid.UUID, markFn func(context.Context) error) error {
	if s.seqEmitter != nil && s.tx != nil {
		return s.tx.WithTx(ctx, func(txCtx context.Context) error {
			if err := markFn(txCtx); err != nil {
				return err
			}
			return s.emitIfRunComplete(txCtx, msg, userID)
		})
	}
	return markFn(ctx)
}

// emitIfRunComplete enqueues sequence.completed when msg was the run's last
// pending dispatch. It runs inside the dispatch transaction so the count
// reflects the just-persisted mark. A message with no sequence (ad-hoc send) is
// a no-op. Because the count runs in-tx on EVERY dispatch (not only run-final
// ones), a CountPendingDispatch error aborts that dispatch and re-sends it next
// tick — wider than a completion-only failure, but bounded by Resend idempotency.
//
// Completion means "the last dispatch emptied the run's queue", not "the queue
// is empty": a step that can never be dispatched (permanently suppressed,
// consent-blocked, or failing non-bounce every tick) stays approved, so the
// count never reaches zero and the run never completes. That matches the
// "finished sending" definition and mirrors the infinite-retry behaviour of
// GetPendingSends.
func (s *Sender) emitIfRunComplete(ctx context.Context, msg seqdomain.OutboundMessage, userID uuid.UUID) error {
	if msg.SequenceID == uuid.Nil {
		return nil
	}
	remaining, err := s.seqRepo.CountPendingDispatch(ctx, msg.ProspectID, msg.SequenceID)
	if err != nil {
		return fmt.Errorf("outbound: sequence-completion count for prospect %s seq %s: %w", msg.ProspectID, msg.SequenceID, err)
	}
	if remaining == 0 {
		return s.seqEmitter.EmitSequenceCompleted(ctx, SequenceCompletion{
			UserID:     userID,
			ProspectID: msg.ProspectID,
			SequenceID: msg.SequenceID,
		})
	}
	return nil
}

// recipientAllowed applies the send guard's per-recipient check, failing
// open only when no guard is configured. Logs and returns false on refusal.
func (s *Sender) recipientAllowed(channel, recipient string, msgID uuid.UUID) bool {
	if s.guard == nil {
		return true
	}
	if ok, reason := s.guard.CheckRecipient(channel, recipient); !ok {
		log.Printf("[outbound][guard] skipping msg %s: %s (%s)", msgID, reason, channel)
		return false
	}
	return true
}

// unsubscribeURL returns the one-click unsubscribe URL for a prospect, and
// false when unsubscribe links are not configured (no secret or no base URL).
func (s *Sender) unsubscribeURL(prospectID uuid.UUID) (string, bool) {
	if s.unsubscribeSecret == "" || s.appBaseURL == "" {
		return "", false
	}
	token := prospectsdomain.SignUnsubscribeToken(prospectID, s.unsubscribeSecret)
	return strings.TrimRight(s.appBaseURL, "/") + "/unsubscribe/" + token, true
}

// unsubscribeEmailHeaders returns the RFC 8058 one-click unsubscribe headers
// for url. List-Unsubscribe-Post signals that a bare POST to the URL performs
// the unsubscribe without further interaction.
func unsubscribeEmailHeaders(url string) map[string]string {
	return map[string]string{
		"List-Unsubscribe":      "<" + url + ">",
		"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
	}
}

// unsubscribeFooter is the visible unsubscribe link appended to the email body.
func unsubscribeFooter(url string) string {
	return `<p style="font-size:12px;color:#888;margin-top:24px">` +
		`Если вы не хотите получать эти письма, <a href="` + url + `">отписаться</a>.</p>`
}

// buildSMTPMessage assembles the raw RFC 822 message: the standard headers,
// any extraHeaders (e.g. List-Unsubscribe), a blank line, then the HTML body.
func buildSMTPMessage(from, to, subject, htmlBody string, extraHeaders map[string]string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	// Sort keys for deterministic header ordering (stable output, testable).
	keys := make([]string, 0, len(extraHeaders))
	for k := range extraHeaders {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "%s: %s\r\n", k, extraHeaders[k])
	}
	b.WriteString("\r\n")
	b.WriteString(htmlBody)
	return []byte(b.String())
}

func NewSender(
	store ConfigStore, ownerID uuid.UUID,
	fallbackKey, fromAddress, appBaseURL string,
	smtpHost, smtpPort, smtpUser, smtpPassword string,
	seqRepo OutboundRepository, prospectRepo ProspectLookup,
	tgRepo TelegramSessionStore,
	tgMessenger TelegramMessenger,
	dialer proxy.ContextDialer,
	httpClient *http.Client,
) *Sender {
	return &Sender{
		store:        store,
		ownerID:      ownerID,
		fallbackKey:  fallbackKey,
		fromAddress:  fromAddress,
		appBaseURL:   appBaseURL,
		smtpHost:     smtpHost,
		smtpPort:     smtpPort,
		smtpUser:     smtpUser,
		smtpPassword: smtpPassword,
		seqRepo:      seqRepo,
		prospectRepo: prospectRepo,
		tgRepo:       tgRepo,
		tgMessenger:  tgMessenger,
		dialer:       dialer,
		httpClient:   httpClient,
	}
}

// SendOneEmailFor dispatches a single ad-hoc email outside the
// sequence outbound pipeline. Resolves the user's SMTP / Resend
// configuration per call (so a config change after Sender startup
// takes effect immediately) and routes through the same SMTP or
// Resend branch as SendPending. Used by the inbox HITL email
// dispatcher for an approved auto-drafted reply.
//
// idempotencyKey is forwarded to the Resend API only — SMTP has no
// idempotency story (see the comment on sendViaSMTPWith). Callers
// SHOULD pass a stable key derived from the PendingReply ID so a
// retry after a transient HTTP failure does not double-deliver.
func (s *Sender) SendOneEmailFor(ctx context.Context, userID uuid.UUID, to, subject, body, idempotencyKey string) error {
	smtpHost, smtpPort, smtpUser, smtpPassword := s.smtpHost, s.smtpPort, s.smtpUser, s.smtpPassword
	fromAddr := s.fromAddress
	apiKey := s.fallbackKey
	// Single GetConfig call resolves both SMTP and Resend for the
	// explicit userID. Calling sendViaResend (which does its own
	// GetConfig keyed on s.ownerID) here would silently fall back to
	// the boot-time owner's Resend key in a multi-tenant deployment;
	// resolve everything up front and pass the key explicitly into
	// the wire-level helper instead.
	if cfg, err := s.store.GetConfig(ctx, userID); err == nil && cfg != nil {
		smtpHost = settingsdomain.ResolveConfig(cfg.SMTPHost, smtpHost)
		smtpPort = settingsdomain.ResolveConfig(cfg.SMTPPort, smtpPort)
		smtpUser = settingsdomain.ResolveConfig(cfg.SMTPUser, smtpUser)
		smtpPassword = settingsdomain.ResolveConfig(cfg.SMTPPassword, smtpPassword)
		apiKey = settingsdomain.ResolveConfig(cfg.ResendAPIKey, apiKey)
		if smtpUser != "" && fromAddr == "" {
			fromAddr = smtpUser
		}
	}
	htmlBody := "<html><body>" + body + "</body></html>"
	// Ad-hoc inbox replies carry no unsubscribe header — they answer an inbound
	// contact, not a cold outbound campaign.
	if smtpHost != "" && smtpUser != "" && smtpPassword != "" {
		return s.sendViaSMTPWith(ctx, smtpHost, smtpPort, smtpUser, smtpPassword, fromAddr, to, subject, htmlBody, nil)
	}
	return s.dispatchToResend(ctx, apiKey, to, subject, htmlBody, idempotencyKey, nil)
}

// SendPending finds all approved email messages ready to send.
// Uses SMTP if configured (DB first, then .env), otherwise falls back to Resend API.
func (s *Sender) SendPending(ctx context.Context) error {
	// Resolve SMTP/Resend settings from DB, fallback to .env
	smtpHost, smtpPort, smtpUser, smtpPassword := s.smtpHost, s.smtpPort, s.smtpUser, s.smtpPassword
	fromAddr := s.fromAddress
	if cfg, err := s.store.GetConfig(ctx, s.ownerID); err == nil {
		smtpHost = settingsdomain.ResolveConfig(cfg.SMTPHost, smtpHost)
		smtpPort = settingsdomain.ResolveConfig(cfg.SMTPPort, smtpPort)
		smtpUser = settingsdomain.ResolveConfig(cfg.SMTPUser, smtpUser)
		smtpPassword = settingsdomain.ResolveConfig(cfg.SMTPPassword, smtpPassword)
		// Use SMTP user as from address if no explicit from set
		if smtpUser != "" && fromAddr == "" {
			fromAddr = smtpUser
		}
	}

	msgs, err := s.seqRepo.GetPendingSends(ctx)
	if err != nil {
		return fmt.Errorf("get pending sends: %w", err)
	}

	// Layer 3: bound the blast radius of a single dispatch tick. An
	// over-threshold batch is held until a mass send is confirmed out-of-band.
	if s.guard != nil {
		if ok, reason := s.guard.CheckBatch(len(msgs)); !ok {
			log.Printf("[outbound][guard] holding batch of %d: %s", len(msgs), reason)
			return nil
		}
	}

	for _, msg := range msgs {
		if msg.Channel == "telegram" {
			s.handleTelegramMessage(ctx, msg)
			continue
		}
		if msg.Channel != "email" {
			continue
		}

		prospect, err := s.prospectRepo.GetProspect(ctx, msg.ProspectID)
		if err != nil {
			log.Printf("[outbound] error fetching prospect %s: %v", msg.ProspectID, err)
			continue
		}
		if prospect == nil || prospect.Email == "" {
			continue
		}
		if !s.recipientAllowed("email", prospect.Email, msg.ID) {
			continue
		}
		if s.isSuppressed(ctx, prospect.UserID, prospectsdomain.SuppressionChannelEmail, prospect.Email, msg.ID) {
			continue
		}
		if !authorizeConsent(prospect, msg.ID) {
			continue
		}

		subject := fmt.Sprintf("%s, сотрудничество с %s", prospect.Name, prospect.Company)
		if prospect.Company == "" {
			subject = fmt.Sprintf("%s, предложение о сотрудничестве", prospect.Name)
		}

		trackingPixel := ""
		if s.appBaseURL != "" {
			trackingPixel = fmt.Sprintf(`<img src="%s/api/track/open/%s" width="1" height="1" style="display:none" />`, s.appBaseURL, msg.ID)
		}

		// Unsubscribe link + RFC 8058 one-click headers, keyed to the prospect.
		footer := ""
		var extraHeaders map[string]string
		if url, ok := s.unsubscribeURL(prospect.ID); ok {
			footer = unsubscribeFooter(url)
			extraHeaders = unsubscribeEmailHeaders(url)
		}

		htmlBody := "<html><body>" + msg.Body + footer + trackingPixel + "</body></html>"

		var sendErr error
		if smtpHost != "" && smtpUser != "" && smtpPassword != "" {
			sendErr = s.sendViaSMTPWith(ctx, smtpHost, smtpPort, smtpUser, smtpPassword, fromAddr, prospect.Email, subject, htmlBody, extraHeaders)
		} else {
			sendErr = s.sendViaResend(ctx, prospect.Email, subject, htmlBody, idempotencyKeyPrefix+msg.ID.String(), extraHeaders)
		}

		if sendErr != nil {
			errStr := sendErr.Error()
			if strings.Contains(errStr, "bounce") || strings.Contains(errStr, "invalid") || strings.Contains(errStr, "rejected") || strings.Contains(errStr, "mailbox") || strings.Contains(errStr, "550") || strings.Contains(errStr, "553") {
				log.Printf("[outbound] bounce for %s (msg %s): %v", prospect.Email, msg.ID, sendErr)
				// Domain entity owns the clock symmetrically with MarkSent —
				// if the state machine rejects the transition, nothing is
				// persisted. The bouncedAt we pass to the repo is what the
				// entity just recorded.
				if err := msg.MarkBounced(time.Now().UTC()); err != nil {
					log.Printf("[outbound] refusing to mark %s bounced: %v", msg.ID, err)
					continue
				}
				if err := s.commitDispatch(ctx, msg, prospect.UserID, func(c context.Context) error {
					return s.seqRepo.MarkBounced(c, msg.ID, *msg.BouncedAt)
				}); err != nil {
					log.Printf("[outbound] failed to persist %s as bounced: %v", msg.ID, err)
				}
				_ = s.prospectRepo.UpdateVerification(ctx, msg.ProspectID, prospectsdomain.VerifyStatusInvalid, 0, `{"bounce":true}`, time.Now().UTC())
				continue
			}
			log.Printf("[outbound] send failed to %s (msg %s): %v", prospect.Email, msg.ID, sendErr)
			continue
		}

		// Validate approved→sent via the domain; the entity owns the clock
		// and SentAt becomes the source of truth the repo persists.
		if err := msg.MarkSent(time.Now().UTC()); err != nil {
			log.Printf("[outbound] refusing to mark %s sent: %v", msg.ID, err)
			continue
		}
		if err := s.commitDispatch(ctx, msg, prospect.UserID, func(c context.Context) error {
			return s.seqRepo.MarkSent(c, msg.ID, *msg.SentAt)
		}); err != nil {
			log.Printf("[outbound] failed to persist %s as sent: %v", msg.ID, err)
			continue
		}

		log.Printf("[outbound] sent email to %s (msg %s)", prospect.Email, msg.ID)
	}

	return nil
}

// sendViaSMTPWith sends email through an SMTP server (mail.ru, Yandex, Gmail, etc.)
//
// UNLIKE sendViaResend, the SMTP path has no retry and no idempotency
// guarantee. SMTP-level dedup would require a stable Message-ID header
// plus cooperation from every receiving MTA's duplicate-detection (or
// our own server-side tracking of which Message-IDs have already been
// SMTP-handed-off). Neither is implemented. A timeout between SMTP
// handoff and ack on this path may result in duplicate delivery on a
// subsequent cron tick that re-fetches the same row. Tracked as a
// known gap; the Resend path (the production default) is safe.
func (s *Sender) sendViaSMTPWith(ctx context.Context, host, port, user, password, from, to, subject, htmlBody string, extraHeaders map[string]string) error {
	if from == "" {
		from = user
	}

	message := buildSMTPMessage(from, to, subject, htmlBody, extraHeaders)

	addr := host + ":" + port
	auth := smtp.PlainAuth("", user, password, host)

	// mail.ru / Yandex require TLS on port 465
	if port == "465" {
		return s.sendSMTPWithTLS(ctx, addr, auth, host, from, to, message)
	}

	// Port 587 uses STARTTLS
	return s.sendSMTPWithSTARTTLS(ctx, addr, auth, host, from, to, message)
}

// sendSMTPWithTLS handles implicit TLS (port 465).
func (s *Sender) sendSMTPWithTLS(ctx context.Context, addr string, auth smtp.Auth, host, from, to string, message []byte) error {
	tlsConfig := &tls.Config{ServerName: host}

	var conn net.Conn
	var err error
	if s.dialer != nil {
		rawConn, dialErr := s.dialer.DialContext(ctx, "tcp", addr)
		if dialErr != nil {
			return fmt.Errorf("smtp proxy dial: %w", dialErr)
		}
		conn = tls.Client(rawConn, tlsConfig)
	} else {
		conn, err = tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("smtp tls dial: %w", err)
		}
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(message); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}

	return client.Quit()
}

// sendSMTPWithSTARTTLS handles port 587 STARTTLS via manual SMTP client (supports proxy dialer).
func (s *Sender) sendSMTPWithSTARTTLS(ctx context.Context, addr string, auth smtp.Auth, host, from, to string, message []byte) error {
	var conn net.Conn
	var err error

	if s.dialer != nil {
		conn, err = s.dialer.DialContext(ctx, "tcp", addr)
	} else {
		conn, err = net.DialTimeout("tcp", addr, 30*time.Second)
	}
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
		return fmt.Errorf("smtp starttls: %w", err)
	}
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(message); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}
	return client.Quit()
}

// tgRateInterval is the minimum interval between Telegram messages to avoid flood.
const tgRateInterval = 90 * time.Second

// handleTelegramMessage sends a single outbound message via personal Telegram account.
func (s *Sender) handleTelegramMessage(ctx context.Context, msg seqdomain.OutboundMessage) {
	if s.tgRepo == nil {
		log.Printf("[outbound] telegram repo not configured, skipping msg %s", msg.ID)
		return
	}

	// Rate limit: max 1 TG message per 90 seconds.
	s.tgRateMu.Lock()
	if since := time.Since(s.tgLastSent); since < tgRateInterval {
		s.tgRateMu.Unlock()
		log.Printf("[outbound] telegram rate limit: %v until next send, skipping msg %s", tgRateInterval-since, msg.ID)
		return
	}
	s.tgRateMu.Unlock()

	phone, sessionData, err := s.tgRepo.GetSession(ctx, s.ownerID.String())
	if err != nil {
		log.Printf("[outbound] error getting TG session: %v", err)
		return
	}
	if len(sessionData) == 0 || phone == "" {
		log.Printf("[outbound] no telegram session configured, skipping msg %s", msg.ID)
		return
	}

	prospect, err := s.prospectRepo.GetProspect(ctx, msg.ProspectID)
	if err != nil {
		log.Printf("[outbound] error fetching prospect %s: %v", msg.ProspectID, err)
		return
	}
	if prospect == nil {
		return
	}
	if s.isSuppressed(ctx, prospect.UserID, prospectsdomain.SuppressionChannelTelegram, prospect.TelegramUsername, msg.ID) {
		return
	}
	if !authorizeConsent(prospect, msg.ID) {
		return
	}

	// Determine targets: try username first (more reliable), then phone
	var targets []string
	if prospect.TelegramUsername != "" {
		targets = append(targets, "@"+prospect.TelegramUsername)
	}
	if prospect.Phone != "" {
		targets = append(targets, prospect.Phone)
	}
	if len(targets) == 0 {
		log.Printf("[outbound] prospect %s has no phone or TG username, skipping msg %s", msg.ProspectID, msg.ID)
		return
	}

	sendCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var lastErr error
	for _, target := range targets {
		if !s.recipientAllowed("telegram", target, msg.ID) {
			continue
		}
		if err := s.tgMessenger.SendMessage(sendCtx, sessionData, target, msg.Body); err != nil {
			log.Printf("[outbound] telegram attempt %s failed (msg %s): %v", target, msg.ID, err)
			lastErr = err
			continue
		}
		lastErr = nil
		log.Printf("[outbound] sent telegram message via %s (msg %s)", target, msg.ID)
		break
	}
	if lastErr != nil {
		log.Printf("[outbound] telegram send failed for all targets (msg %s): %v", msg.ID, lastErr)
		return
	}

	s.tgRateMu.Lock()
	s.tgLastSent = time.Now()
	s.tgRateMu.Unlock()

	if err := msg.MarkSent(time.Now().UTC()); err != nil {
		log.Printf("[outbound] refusing to mark telegram %s sent: %v", msg.ID, err)
		return
	}
	if err := s.commitDispatch(ctx, msg, prospect.UserID, func(c context.Context) error {
		return s.seqRepo.MarkSent(c, msg.ID, *msg.SentAt)
	}); err != nil {
		log.Printf("[outbound] failed to persist telegram %s as sent: %v", msg.ID, err)
		return
	}

	log.Printf("[outbound] sent telegram message to %s (msg %s)", prospect.Phone, msg.ID)
}

// resendMaxAttempts caps retries at 3 — first try + two backoffs. Any
// more and the cron tick gets stuck behind a single sticky 5xx; any
// less and one transient blip surfaces as a hard send failure.
const resendMaxAttempts = 3

// resendInitialBackoff is the wait before the second attempt; doubled
// for the third. Total worst-case wait = 200 + 400 = 600 ms (plus up
// to ~50% jitter), well under the cron tick budget.
const resendInitialBackoff = 200 * time.Millisecond

// idempotencyKeyPrefix namespaces the Resend Idempotency-Key so it is
// identifiable in the Resend dashboard and so a future second caller
// (e.g. transactional emails outside the outbound cron) cannot
// collide with cron-row keys for free.
const idempotencyKeyPrefix = "outbound:"

// sendViaResend sends email through the Resend API using raw HTTP.
// idempotencyKey is forwarded as the "Idempotency-Key" header so that
// retries of the same outbound row collapse to a single delivery on
// Resend's side (https://resend.com/docs/api-reference/idempotency).
// An empty key is allowed but unsafe — callers should always pass a
// stable per-message value (e.g. "outbound:<message_id>").
//
// Retries up to resendMaxAttempts on transport errors and 5xx
// responses with exponential backoff. 4xx is treated as terminal —
// same body + same key cannot succeed by trying again.
func (s *Sender) sendViaResend(ctx context.Context, to, subject, htmlBody, idempotencyKey string, extraHeaders map[string]string) error {
	apiKey := s.fallbackKey
	if cfg, err := s.store.GetConfig(ctx, s.ownerID); err == nil {
		apiKey = settingsdomain.ResolveConfig(cfg.ResendAPIKey, apiKey)
	}
	return s.dispatchToResend(ctx, apiKey, to, subject, htmlBody, idempotencyKey, extraHeaders)
}

// dispatchToResend is the wire-level Resend HTTP call factored out of
// sendViaResend so multi-tenant callers (SendOneEmailFor) can resolve
// the API key for an explicit userID and pass it down without
// triggering a second GetConfig keyed on s.ownerID.
func (s *Sender) dispatchToResend(ctx context.Context, apiKey, to, subject, htmlBody, idempotencyKey string, extraHeaders map[string]string) error {
	if apiKey == "" {
		return ErrNoResendAPIKey
	}

	payload := map[string]interface{}{
		"from":    s.fromAddress,
		"to":      []string{to},
		"subject": subject,
		"html":    htmlBody,
	}
	if len(extraHeaders) > 0 {
		payload["headers"] = extraHeaders
	}
	body, _ := json.Marshal(payload)

	client := s.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	var lastErr error
	backoff := resendInitialBackoff
	for attempt := 1; attempt <= resendMaxAttempts; attempt++ {
		// New Request per attempt — bytes.Reader has an internal
		// cursor that is exhausted by the previous client.Do.
		req, err := http.NewRequestWithContext(ctx, "POST", resendAPIURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("resend request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("resend send (attempt %d): %w", attempt, err)
			if attempt == resendMaxAttempts {
				return lastErr
			}
			if werr := waitWithCtx(ctx, jitter(backoff)); werr != nil {
				return werr
			}
			backoff *= 2
			continue
		}
		// Drain the response body before Close so the underlying TCP
		// connection can be returned to the keep-alive pool. Resend
		// 5xx may carry a hundred-byte JSON error envelope; without
		// the drain the transport opens a fresh TCP+TLS handshake on
		// every retry attempt.
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		// 429 — transient (rate-limit). Same Idempotency-Key is safe
		// to retry; Resend will dedup if the original eventually
		// landed. Handle BEFORE the generic 4xx-terminal branch.
		retryable := (resp.StatusCode >= 500 && resp.StatusCode < 600) || resp.StatusCode == http.StatusTooManyRequests
		if retryable {
			lastErr = &ResendAPIError{StatusCode: resp.StatusCode}
			if attempt == resendMaxAttempts {
				return lastErr
			}
			if werr := waitWithCtx(ctx, jitter(backoff)); werr != nil {
				return werr
			}
			backoff *= 2
			continue
		}
		if resp.StatusCode >= 400 {
			// 4xx (other than 429) — client error. Same body + same
			// Idempotency-Key will fail the same way; retry burns
			// rate-limit budget without changing the outcome.
			return &ResendAPIError{StatusCode: resp.StatusCode}
		}
		return nil
	}
	return lastErr
}

// jitter spreads backoffs across [d, d*1.5) so multiple senders (or
// tenants) restarting after a Resend outage do not synchronise on the
// retry edge and re-thunder the herd.
func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	return d + time.Duration(rand.Int64N(int64(d)/2))
}

// waitWithCtx sleeps for d unless the context is cancelled first.
// Lets the retry loop honour cron shutdown without leaving a goroutine
// blocked in the middle of a backoff.
func waitWithCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
