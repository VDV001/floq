package audit

import (
	"context"
	"log/slog"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/audit/domain"
)

// RecordingProvider wraps an ai.Provider (and, if the inner one supports
// it, ai.VisionProvider) and hands every call's metadata off to a
// Recorder. The decorator pulls attribution out of the request ctx via
// CallMetaFromContext, computes cost from the audit pricing table, and
// fires-and-forgets a Record call — Recorder is expected to be
// non-blocking.
//
// Inner failures propagate verbatim: this is a passive observer, not a
// retry or fallback layer. The audit row is recorded for both success
// and error outcomes so the spend distribution stays honest (failed-
// but-billed calls do happen with some providers).
type RecordingProvider struct {
	inner    ai.Provider
	recorder domain.Recorder
	logger   *slog.Logger
	observe  func(*domain.Entry)
}

// RecordingOption configures a RecordingProvider at construction.
type RecordingOption func(*RecordingProvider)

// WithObserver registers a side-channel callback invoked with every
// successfully-constructed Entry, just before it is handed to the
// Recorder. Used to feed real-time metrics (Prometheus) without coupling
// the audit layer to the metrics package — the observer fires for both
// success and error outcomes, and regardless of whether the async
// recorder later persists or drops the row. It must be non-blocking.
func WithObserver(fn func(*domain.Entry)) RecordingOption {
	return func(r *RecordingProvider) { r.observe = fn }
}

// Compile-time assertions: RecordingProvider always satisfies Provider;
// it also satisfies VisionProvider, but degrades gracefully via
// ErrVisionUnsupported when the wrapped Provider does not.
var (
	_ ai.Provider       = (*RecordingProvider)(nil)
	_ ai.VisionProvider = (*RecordingProvider)(nil)
)

// NewRecordingProvider wires the decorator. Pass nil logger to use
// slog.Default(). The recorder is mandatory — there is no sensible
// "no-op" mode for the audit layer.
func NewRecordingProvider(inner ai.Provider, recorder domain.Recorder, logger *slog.Logger, opts ...RecordingOption) *RecordingProvider {
	if logger == nil {
		logger = slog.Default()
	}
	rp := &RecordingProvider{inner: inner, recorder: recorder, logger: logger}
	for _, opt := range opts {
		opt(rp)
	}
	return rp
}

func (r *RecordingProvider) Name() string { return r.inner.Name() }

func (r *RecordingProvider) Complete(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResult, error) {
	start := time.Now()
	resp, err := r.inner.Complete(ctx, req)
	r.record(ctx, resp, err, time.Since(start))
	return resp, err
}

// AnalyzeImage routes through the inner provider's VisionProvider
// implementation when present; returns ai.ErrVisionUnsupported when
// the wrapped provider is text-only. The audit row is still recorded
// on success and on a hard provider error — but skipped when the
// vision capability itself is missing (no AI call happened, nothing
// to audit).
func (r *RecordingProvider) AnalyzeImage(ctx context.Context, imageData []byte, mimeType, prompt string) (*ai.CompletionResult, error) {
	vp, ok := r.inner.(ai.VisionProvider)
	if !ok {
		return nil, ai.ErrVisionUnsupported
	}
	start := time.Now()
	resp, err := vp.AnalyzeImage(ctx, imageData, mimeType, prompt)
	r.record(ctx, resp, err, time.Since(start))
	return resp, err
}

// record builds the audit Entry from the call outcome and hands it to
// the Recorder. Missing CallMeta is logged at warn and the entry is
// dropped — without attribution the row is useless. Domain validation
// failures (e.g. an unknown request_type) are also warn-logged; we
// never panic the AI hot path on an audit problem.
func (r *RecordingProvider) record(ctx context.Context, resp *ai.CompletionResult, callErr error, latency time.Duration) {
	meta, ok := domain.CallMetaFromContext(ctx)
	if !ok {
		r.logger.WarnContext(ctx, "audit: AI call missing meta context, skipping audit row",
			"provider", r.inner.Name())
		return
	}

	status := domain.StatusSuccess
	errMsg := ""
	if callErr != nil {
		status = domain.StatusError
		// First strip PII patterns (emails, phones, API keys) — some
		// providers quote the offending prompt fragment verbatim in
		// 4xx errors. Then cap at 256 bytes so a runaway SDK can't
		// stick entire payloads into a persistent column.
		errMsg = truncate(sanitizeErrorMessage(callErr.Error()), 256)
	}

	var (
		model           string
		input, output   int
	)
	if resp != nil {
		model = resp.Model
		input = resp.Usage.InputTokens
		output = resp.Usage.OutputTokens
	}
	if model == "" {
		// On a hard provider error the inner adapter may not have
		// resolved the concrete model. "unknown" keeps the row valid
		// (model is required) and signals the missing attribution to
		// downstream cost reports.
		model = "unknown"
	}

	cost, _ := CostMicroUSD(r.inner.Name(), model, input, output)

	entry, entryErr := domain.NewEntry(domain.EntryParams{
		UserID:       meta.UserID,
		LeadID:       meta.LeadID,
		ProspectID:   meta.ProspectID,
		RequestType:  meta.RequestType,
		Provider:     r.inner.Name(),
		Model:        model,
		InputTokens:  input,
		OutputTokens: output,
		CostUSDMicro: cost,
		LatencyMS:    int(latency.Milliseconds()),
		Status:       status,
		ErrorMessage: errMsg,
	})
	if entryErr != nil {
		r.logger.WarnContext(ctx, "audit: entry construction failed, skipping row",
			"err", entryErr, "provider", r.inner.Name(), "model", model)
		return
	}
	if r.observe != nil {
		// Real-time metrics hook — fires for every real call (success or
		// error), before the async Record so a saturated recorder cannot
		// suppress the "call happened" signal.
		r.observe(entry)
	}
	r.recorder.Record(ctx, entry)
}

// piiPatterns strip the most common user-content tokens that providers
// echo back in their error strings (we've seen OpenAI's 4xx responses
// quote the offending prompt fragment verbatim). Run before truncation
// so the redaction tokens don't get cut mid-substitution.
//
// Residual-risk note (also in docs/audit-log.md privacy section): we
// scrub the high-frequency classes (email, phone, sk-/Bearer/AWS-key
// prefixes, basic-auth URLs, raw IPv4). Free-form names, addresses,
// and structured CRM identifiers (lead IDs, ticket numbers) are NOT
// redacted — the assumption is that providers do not echo those in
// error strings. Add patterns here if a real incident proves otherwise.
var piiPatterns = []*regexp.Regexp{
	// Email addresses
	regexp.MustCompile(`[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}`),
	// E.164-ish phone numbers (+ followed by 7-15 digits, optional spaces/dashes inside)
	regexp.MustCompile(`\+\d[\d\s\-()]{6,18}\d`),
	// Bearer / sk- API keys / AWS access keys / generic Authorization headers
	regexp.MustCompile(`sk-[A-Za-z0-9_\-]{16,}`),
	regexp.MustCompile(`Bearer\s+[A-Za-z0-9._\-]{16,}`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`(?i)authorization:\s*[A-Za-z0-9._\-=]{16,}`),
	// Basic-auth URLs (creds in the userinfo segment)
	regexp.MustCompile(`https?://[^/\s:@]+:[^/\s@]+@`),
	// Bare IPv4 (rough — won't catch IPv6, but covers the common case)
	regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`),
}

func sanitizeErrorMessage(s string) string {
	for _, p := range piiPatterns {
		s = p.ReplaceAllString(s, "[REDACTED]")
	}
	return s
}

// truncate returns s when its byte length fits max, otherwise the
// longest valid-UTF-8 prefix that, with an appended '…' marker, stays
// within max bytes. Walks back from the cap to a rune boundary so a
// non-ASCII error (Russian / Chinese / emoji) does not split mid-
// codepoint and produce invalid UTF-8 downstream. max ≤ 0 returns "".
// When max is too small to fit even the marker, returns the longest
// rune-aligned prefix of s that fits — never invalid UTF-8.
func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	const marker = "…"
	if max < len(marker) {
		// No room for the ellipsis — fall back to a rune-aligned
		// prefix of s, which keeps the output valid UTF-8.
		return runeAlignedPrefix(s, max)
	}
	return runeAlignedPrefix(s, max-len(marker)) + marker
}

func runeAlignedPrefix(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if limit >= len(s) {
		return s
	}
	// Walk back to the previous rune start within [0, limit].
	for limit > 0 && !utf8.RuneStart(s[limit]) {
		limit--
	}
	return s[:limit]
}
