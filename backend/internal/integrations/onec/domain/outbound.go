package domain

import (
	"strings"

	"github.com/google/uuid"
)

// CounterpartyDraft is the value object Floq pushes to 1C to create a
// counterparty (контрагент) when a lead is qualified. It is built from
// Floq-side data, not from a 1C event, so it carries no external id yet —
// 1C assigns one on creation. The factory enforces the single invariant
// that 1C needs to identify the party: at least a name or an email.
type CounterpartyDraft struct {
	Name    string
	Email   string
	Company string
}

// NewCounterpartyDraft trims its inputs and rejects a draft that carries
// neither a name nor an email (1C cannot create an anonymous counterparty).
func NewCounterpartyDraft(name, email, company string) (*CounterpartyDraft, error) {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	company = strings.TrimSpace(company)
	if name == "" && email == "" {
		return nil, ErrEmptyCounterparty
	}
	return &CounterpartyDraft{Name: name, Email: email, Company: company}, nil
}

// NewOutboundSyncRecord builds a ledger entry for an action Floq pushed to 1C.
// Unlike the inbound NewSyncRecord, the dedup key is Floq-side: externalType is
// the 1C object kind being created (e.g. "counterparty") and externalID is a
// stable Floq identity for the source entity, so a re-qualification dedups
// instead of creating a duplicate. The status is set by the caller to reflect
// the POST result (Processed / Error). PayloadHash stays empty — there is no
// inbound 1C payload on the outbound path.
func NewOutboundSyncRecord(userID uuid.UUID, externalID, externalType string, kind EventKind, status SyncStatus) (*SyncRecord, error) {
	if userID == uuid.Nil {
		return nil, ErrNilUser
	}
	if externalID == "" {
		return nil, ErrEmptyExternalID
	}
	if externalType == "" {
		return nil, ErrEmptyExternalType
	}
	if !kind.IsValid() {
		return nil, ErrInvalidEventKind
	}
	if !status.IsValid() {
		return nil, ErrInvalidSyncStatus
	}
	return &SyncRecord{
		ID:           uuid.New(),
		UserID:       userID,
		ExternalID:   externalID,
		ExternalType: externalType,
		Direction:    DirectionOutbound,
		Kind:         kind,
		Status:       status,
		PayloadHash:  "",
	}, nil
}
