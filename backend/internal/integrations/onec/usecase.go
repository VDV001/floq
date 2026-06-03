package onec

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/google/uuid"
)

// ErrUnresolvableKind is returned when an event carries no kind and no mapping
// rule matches its external type — Floq cannot classify it, so it is rejected
// (the handler maps this to 422) rather than recorded with an unknown kind.
var ErrUnresolvableKind = errors.New("onec: cannot resolve event kind (no kind and no mapping rule)")

// RawInboundEvent is the unparsed event as received from 1C. Kind is optional:
// when empty it is derived from the user's mapping by ExternalType (hybrid
// resolution); when present it wins.
type RawInboundEvent struct {
	ExternalID   string
	ExternalType string
	Kind         string
	Payload      []byte
}

// UseCase orchestrates inbound 1C event handling: resolve the canonical kind,
// record the event idempotently, and apply the mapped domain action.
type UseCase struct {
	store   SyncStore
	mapping MappingStore // optional; nil → no mapping/application
	applier EventApplier // optional; nil → record only
	logger  *slog.Logger
}

// Option configures a UseCase.
type Option func(*UseCase)

// WithLogger sets the logger used for drift/anomaly reporting.
func WithLogger(l *slog.Logger) Option { return func(u *UseCase) { u.logger = l } }

// WithMapping enables mapping-based kind resolution and email extraction.
func WithMapping(m MappingStore) Option { return func(u *UseCase) { u.mapping = m } }

// WithApplier enables applying mapped events to domain actions.
func WithApplier(a EventApplier) Option { return func(u *UseCase) { u.applier = a } }

// NewUseCase wires the use case over a SyncStore and optional collaborators.
func NewUseCase(store SyncStore, opts ...Option) *UseCase {
	u := &UseCase{store: store, logger: slog.Default()}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

// ProcessResult reports the outcome of handling one inbound event.
type ProcessResult struct {
	// Deduped is true when the event was already seen and this call was a
	// no-op — the webhook handler still returns 200 for replays.
	Deduped bool
	// PayloadDrifted is true when a replay arrived with changed content.
	PayloadDrifted bool
}

// ProcessInboundEvent resolves, records, and applies one 1C event for a user.
//
//  1. Resolve the canonical kind: explicit Kind wins; otherwise derive from the
//     mapping rule for ExternalType. Unresolvable → ErrUnresolvableKind.
//  2. Record idempotently (replays are no-ops, drift is logged).
//  3. On a fresh insert, apply the mapped domain action via the applier. The
//     event is already durably recorded, so an apply failure is logged rather
//     than propagated (a 1C retry would only duplicate; reconciliation re-applies).
func (u *UseCase) ProcessInboundEvent(ctx context.Context, userID uuid.UUID, in RawInboundEvent) (ProcessResult, error) {
	rule, hasRule, lookupErr := u.lookupRule(ctx, userID, in.ExternalType)

	kind, err := resolveKind(in.Kind, rule, hasRule, lookupErr)
	if err != nil {
		return ProcessResult{}, err
	}

	ev, err := domain.NewExternalEvent(in.ExternalID, in.ExternalType, kind, in.Payload)
	if err != nil {
		return ProcessResult{}, err
	}
	rec, err := domain.NewSyncRecord(userID, ev, domain.DirectionInbound)
	if err != nil {
		return ProcessResult{}, err
	}
	out, err := u.store.InsertSyncRecord(ctx, rec)
	if err != nil {
		return ProcessResult{}, err
	}
	if out.PayloadDrifted {
		u.logger.Warn("onec: replayed event arrived with changed payload; not re-applied",
			"user_id", userID, "external_id", ev.ExternalID, "external_type", ev.ExternalType, "kind", kind.String())
	}

	// Apply only on a fresh insert, and only when a rule exists (we need its
	// fields to extract the counterparty). Replays are not re-applied.
	if out.Inserted && hasRule && u.applier != nil {
		u.apply(ctx, userID, kind, rule, in.Payload)
	}

	return ProcessResult{Deduped: !out.Inserted, PayloadDrifted: out.PayloadDrifted}, nil
}

// lookupRule resolves the mapping rule for an external type. A missing active
// config (ErrMappingNotFound) is not an error — it just means no rule. Any other
// error (transient DB failure) is returned so the caller can decide whether to
// fail the request rather than silently treat it as "unmapped".
func (u *UseCase) lookupRule(ctx context.Context, userID uuid.UUID, externalType string) (domain.MappingRule, bool, error) {
	if u.mapping == nil {
		return domain.MappingRule{}, false, nil
	}
	cfg, err := u.mapping.GetActiveMappingConfig(ctx, userID)
	if errors.Is(err, ErrMappingNotFound) {
		return domain.MappingRule{}, false, nil
	}
	if err != nil {
		return domain.MappingRule{}, false, err
	}
	rule, ok := cfg.Resolve(externalType)
	return rule, ok, nil
}

// resolveKind implements hybrid resolution: an explicit kind always wins (so a
// classifiable event survives a mapping outage). Otherwise the kind must come
// from the mapping: a transient lookup error propagates (→ 500, retryable),
// genuinely no rule → ErrUnresolvableKind (→ 422).
func resolveKind(explicit string, rule domain.MappingRule, hasRule bool, lookupErr error) (domain.EventKind, error) {
	if explicit != "" {
		return domain.ParseEventKind(explicit) // invalid → ErrInvalidEventKind
	}
	if lookupErr != nil {
		return "", lookupErr
	}
	if hasRule {
		return rule.Kind, nil
	}
	return "", ErrUnresolvableKind
}

// apply routes a recorded event to its domain action. Apply failures are logged,
// not returned — the event is already persisted.
func (u *UseCase) apply(ctx context.Context, userID uuid.UUID, kind domain.EventKind, rule domain.MappingRule, payload []byte) {
	email := extractField(payload, rule.EmailField)
	var err error
	switch kind {
	case domain.EventKindPayment:
		err = u.applier.HandlePayment(ctx, userID, email)
	case domain.EventKindCounterpartyCreated:
		err = u.applier.HandleCounterpartyCreated(ctx, userID, email,
			extractField(payload, rule.NameField), extractField(payload, rule.CompanyField))
	case domain.EventKindOrderStatus:
		err = u.applier.HandleOrderStatus(ctx, userID, email)
	case domain.EventKindShipment:
		err = u.applier.HandleShipment(ctx, userID, email)
	}
	if err != nil {
		u.logger.Warn("onec: applying event failed; recorded but not applied",
			"user_id", userID, "kind", kind.String(), "err", err)
	}
}

// extractField pulls a string value from a raw JSON payload by key. Missing key,
// empty field name, or non-string value yields "".
func extractField(payload []byte, field string) string {
	if field == "" {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(payload, &m); err != nil {
		return ""
	}
	raw, ok := m[field]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}
