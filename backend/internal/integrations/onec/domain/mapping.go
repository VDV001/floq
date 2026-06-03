package domain

import (
	"errors"

	"github.com/google/uuid"
)

// Mapping domain errors.
var (
	ErrNoRules               = errors.New("onec: mapping config needs at least one rule")
	ErrDuplicateExternalType = errors.New("onec: duplicate external type in mapping rules")
)

// MappingRule maps one 1C document type onto a canonical EventKind, plus where
// in the raw payload to find the counterparty's email — the key Floq uses to
// link the event to an existing lead/prospect. This is what makes the
// integration configuration-agnostic: a УТ install and an ERP install can send
// different ExternalType strings that both map to EventKindPayment.
type MappingRule struct {
	ExternalType string    // 1C document/object type, e.g. "Документ.ОплатаПокупателя"
	Kind         EventKind // canonical Floq event
	EmailField   string    // JSON key in payload holding the counterparty email (optional)
}

// MappingConfig is a user's full set of 1C→Floq mapping rules. Built and
// validated through NewMappingConfig; rules are unique by ExternalType so
// resolution is unambiguous.
type MappingConfig struct {
	UserID uuid.UUID
	Rules  []MappingRule
}

// NewMappingConfig validates and constructs a MappingConfig. It requires a
// non-nil user, at least one rule, and each rule to carry a non-empty external
// type and a valid kind, with no duplicate external types.
func NewMappingConfig(userID uuid.UUID, rules []MappingRule) (*MappingConfig, error) {
	if userID == uuid.Nil {
		return nil, ErrNilUser
	}
	if len(rules) == 0 {
		return nil, ErrNoRules
	}
	seen := make(map[string]struct{}, len(rules))
	for _, r := range rules {
		if r.ExternalType == "" {
			return nil, ErrEmptyExternalType
		}
		if !r.Kind.IsValid() {
			return nil, ErrInvalidEventKind
		}
		if _, dup := seen[r.ExternalType]; dup {
			return nil, ErrDuplicateExternalType
		}
		seen[r.ExternalType] = struct{}{}
	}
	return &MappingConfig{UserID: userID, Rules: rules}, nil
}

// Resolve returns the rule for a 1C external type, or ok=false when no rule
// matches (the event is then ignored as unmapped).
func (c *MappingConfig) Resolve(externalType string) (MappingRule, bool) {
	for _, r := range c.Rules {
		if r.ExternalType == externalType {
			return r, true
		}
	}
	return MappingRule{}, false
}
