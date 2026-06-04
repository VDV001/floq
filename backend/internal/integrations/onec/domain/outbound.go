package domain

import (
	"strings"

	"github.com/google/uuid"
)

// AuthType is how Floq authenticates outbound calls to a tenant's 1C endpoint.
// Mirrors the onec_credentials.auth_type CHECK (basic | token).
type AuthType string

const (
	AuthTypeBasic AuthType = "basic" // HTTP Basic (base64 user:pass in AuthSecret)
	AuthTypeToken AuthType = "token" // Bearer token in AuthSecret
)

// IsValid reports whether a is a known auth type.
func (a AuthType) IsValid() bool {
	return a == AuthTypeBasic || a == AuthTypeToken
}

// ParseAuthType converts a wire string into an AuthType, rejecting unknown
// values with ErrInvalidAuthType.
func ParseAuthType(s string) (AuthType, error) {
	a := AuthType(s)
	if !a.IsValid() {
		return "", ErrInvalidAuthType
	}
	return a, nil
}

// OutboundCredentials is the per-user connection Floq uses to push objects to a
// 1C endpoint. BaseURL is the OData/HTTP-service root; AuthSecret is the opaque
// credential interpreted per AuthType. The factory enforces a usable endpoint:
// a non-empty base URL (trailing slash trimmed so path joins are predictable)
// and a valid auth type.
type OutboundCredentials struct {
	BaseURL    string
	AuthType   AuthType
	AuthSecret string
}

// NewOutboundCredentials validates and normalises an outbound connection.
func NewOutboundCredentials(baseURL string, authType AuthType, authSecret string) (*OutboundCredentials, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, ErrEmptyBaseURL
	}
	if !authType.IsValid() {
		return nil, ErrInvalidAuthType
	}
	return &OutboundCredentials{BaseURL: baseURL, AuthType: authType, AuthSecret: authSecret}, nil
}

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
