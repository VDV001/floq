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
	// Applied is true when this call ran the mapped domain action and it
	// succeeded — either a fresh event or the recovery of a recorded-but-unapplied
	// one. Reconciliation counts this to know what it recovered.
	Applied bool
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

	// Apply when the action hasn't yet succeeded and a rule exists (we need its
	// fields to extract the counterparty). That covers a fresh insert AND a
	// replay of a recorded-but-unapplied event — the reconciliation recovery
	// path. A replay of an already-processed event is skipped.
	applied := false
	if hasRule && u.applier != nil && !out.AlreadyProcessed {
		if applyErr := u.apply(ctx, userID, kind, rule, in.Payload); applyErr == nil {
			applied = true
			// Domain actions are idempotent, so a missed mark only risks a
			// harmless re-apply on the next pass — log, don't fail.
			if mErr := u.store.MarkProcessed(ctx, rec); mErr != nil {
				u.logger.Warn("onec: action applied but marking processed failed; may re-apply",
					"user_id", userID, "external_id", ev.ExternalID, "err", mErr)
			}
		}
	}

	return ProcessResult{Deduped: !out.Inserted, PayloadDrifted: out.PayloadDrifted, Applied: applied}, nil
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

// apply routes a recorded event to its domain action and returns the action's
// error. A failure is logged here and returned so the caller leaves the record
// unprocessed (reconciliation will retry); the event itself stays persisted.
func (u *UseCase) apply(ctx context.Context, userID uuid.UUID, kind domain.EventKind, rule domain.MappingRule, payload []byte) error {
	fields := parseStringFields(payload)
	email := fields[rule.EmailField]
	var err error
	switch kind {
	case domain.EventKindPayment:
		err = u.applier.HandlePayment(ctx, userID, email)
	case domain.EventKindCounterpartyCreated:
		err = u.applier.HandleCounterpartyCreated(ctx, userID, email, fields[rule.NameField], fields[rule.CompanyField])
	case domain.EventKindOrderStatus:
		err = u.applier.HandleOrderStatus(ctx, userID, email)
	case domain.EventKindShipment:
		err = u.applier.HandleShipment(ctx, userID, email)
	}
	if err != nil {
		u.logger.Warn("onec: applying event failed; recorded but left unprocessed for reconciliation",
			"user_id", userID, "kind", kind.String(), "err", err)
	}
	return err
}

// parseStringFields flattens a raw JSON object's string-valued top-level keys
// into a map (parsed once per event). A non-object/invalid payload yields an
// empty map, so every field lookup safely returns "". Empty field names in a
// rule then resolve to "" via the missing-key path.
func parseStringFields(payload []byte) map[string]string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		var s string
		if json.Unmarshal(v, &s) == nil {
			out[k] = s
		}
	}
	return out
}
