// Package domain holds the bounded-context model for the 1C integration:
// the inbound event vocabulary (EventKind), the value object that wraps a
// raw 1C event (ExternalEvent), and the sync ledger entry (SyncRecord) used
// for idempotent dedup. All invariants are enforced through the factories
// here — no struct must be assembled directly outside this package.
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"
)

// Domain errors. Declared as package vars so callers can use errors.Is.
var (
	ErrInvalidEventKind  = errors.New("onec: invalid event kind")
	ErrEmptyExternalID   = errors.New("onec: external id is required")
	ErrEmptyExternalType = errors.New("onec: external type is required")
	ErrInvalidDirection  = errors.New("onec: invalid sync direction")
	ErrNilUser           = errors.New("onec: user id is required")
	ErrNilEvent          = errors.New("onec: external event is required")
	ErrInvalidSyncStatus = errors.New("onec: invalid sync status")
	ErrEmptyCounterparty = errors.New("onec: counterparty needs at least a name or email")
	ErrInvalidAuthType   = errors.New("onec: invalid auth type")
	ErrEmptyBaseURL      = errors.New("onec: base url is required")
)

// EventKind is the ubiquitous-language enum of 1C events Floq reacts to.
// The 1C side maps its own document types onto these kinds; Floq only ever
// deals with this closed set, never raw 1C document names.
type EventKind string

const (
	EventKindPayment             EventKind = "payment"              // счёт оплачен
	EventKindCounterpartyCreated EventKind = "counterparty_created" // новый контрагент
	EventKindOrderStatus         EventKind = "order_status"         // изменился статус заказа/сделки
	EventKindShipment            EventKind = "shipment"             // отгрузка / акт
)

// IsValid reports whether k is one of the known event kinds.
func (k EventKind) IsValid() bool {
	switch k {
	case EventKindPayment, EventKindCounterpartyCreated, EventKindOrderStatus, EventKindShipment:
		return true
	default:
		return false
	}
}

// String returns the wire representation of the kind.
func (k EventKind) String() string { return string(k) }

// ParseEventKind converts a wire string into an EventKind, rejecting unknown
// values with ErrInvalidEventKind.
func ParseEventKind(s string) (EventKind, error) {
	k := EventKind(s)
	if !k.IsValid() {
		return "", ErrInvalidEventKind
	}
	return k, nil
}

// SyncDirection distinguishes events received from 1C (inbound) from actions
// Floq pushes to 1C (outbound). Both share the same ledger.
type SyncDirection string

const (
	DirectionInbound  SyncDirection = "inbound"
	DirectionOutbound SyncDirection = "outbound"
)

// IsValid reports whether d is a known direction.
func (d SyncDirection) IsValid() bool {
	return d == DirectionInbound || d == DirectionOutbound
}

// SyncStatus is the lifecycle state of a ledger entry. Inbound capture lands as
// Received (#106). Outbound pushes to 1C (#108) record their result directly:
// Processed on a successful POST, Error when 1C rejected/was unreachable.
type SyncStatus string

const (
	SyncStatusReceived  SyncStatus = "received"  // принято от 1С, ещё не применено
	SyncStatusProcessed SyncStatus = "processed" // действие успешно выполнено в 1С
	SyncStatusError     SyncStatus = "error"     // 1С отверг/недоступен
)

// IsValid reports whether s is a known sync status.
func (s SyncStatus) IsValid() bool {
	switch s {
	case SyncStatusReceived, SyncStatusProcessed, SyncStatusError:
		return true
	default:
		return false
	}
}

// ExternalEvent is a value object wrapping a single 1C event. ExternalID +
// ExternalType form the natural dedup key (a 1C object is uniquely a
// (type, id) pair). Payload is the raw 1C body, kept opaque at this layer —
// interpretation happens in the mapping layer (#107).
type ExternalEvent struct {
	ExternalID   string
	ExternalType string
	Kind         EventKind
	Payload      []byte
}

// NewExternalEvent validates and constructs an ExternalEvent. It rejects an
// empty external id/type and an unknown kind.
func NewExternalEvent(externalID, externalType string, kind EventKind, payload []byte) (*ExternalEvent, error) {
	if externalID == "" {
		return nil, ErrEmptyExternalID
	}
	if externalType == "" {
		return nil, ErrEmptyExternalType
	}
	if !kind.IsValid() {
		return nil, ErrInvalidEventKind
	}
	return &ExternalEvent{
		ExternalID:   externalID,
		ExternalType: externalType,
		Kind:         kind,
		Payload:      payload,
	}, nil
}

// SyncRecord is the ledger entry proving an event was seen, enabling
// idempotent processing: the repository's UNIQUE (user_id, external_id,
// external_type) constraint plus this record means a replayed webhook is a
// no-op. PayloadHash lets a future reconciliation detect a changed payload
// under the same external id.
type SyncRecord struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	ExternalID   string
	ExternalType string
	Direction    SyncDirection
	Kind         EventKind
	Status       SyncStatus
	PayloadHash  string
}

// NewSyncRecord builds a fresh ledger entry in the Received state from a
// validated event. It enforces a non-nil user, a non-nil event and a legal
// direction, and computes the payload hash up front.
func NewSyncRecord(userID uuid.UUID, ev *ExternalEvent, direction SyncDirection) (*SyncRecord, error) {
	if userID == uuid.Nil {
		return nil, ErrNilUser
	}
	if ev == nil {
		return nil, ErrNilEvent
	}
	if !direction.IsValid() {
		return nil, ErrInvalidDirection
	}
	return &SyncRecord{
		ID:           uuid.New(),
		UserID:       userID,
		ExternalID:   ev.ExternalID,
		ExternalType: ev.ExternalType,
		Direction:    direction,
		Kind:         ev.Kind,
		Status:       SyncStatusReceived,
		PayloadHash:  hashPayload(ev.Payload),
	}, nil
}

// hashPayload returns a hex SHA-256 of the raw payload, used to detect
// content drift on replays.
func hashPayload(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
