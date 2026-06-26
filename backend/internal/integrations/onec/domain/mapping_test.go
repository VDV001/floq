package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func rule(extType string, kind EventKind) MappingRule {
	return MappingRule{ExternalType: extType, Kind: kind, EmailField: "counterparty_email"}
}

func TestNewMappingConfig(t *testing.T) {
	user := uuid.New()
	tests := []struct {
		name    string
		userID  uuid.UUID
		rules   []MappingRule
		wantErr error
	}{
		{
			name:   "valid",
			userID: user,
			rules: []MappingRule{
				rule("Документ.ОплатаПокупателя", EventKindPayment),
				rule("Справочник.Контрагенты", EventKindCounterpartyCreated),
			},
			wantErr: nil,
		},
		{"nil user", uuid.Nil, []MappingRule{rule("Документ.Оплата", EventKindPayment)}, ErrNilUser},
		{"no rules", user, nil, ErrNoRules},
		{"empty external type", user, []MappingRule{rule("", EventKindPayment)}, ErrEmptyExternalType},
		{"invalid kind", user, []MappingRule{rule("Документ", EventKind("bad"))}, ErrInvalidEventKind},
		{
			name:   "duplicate external type",
			userID: user,
			rules: []MappingRule{
				rule("Документ.Оплата", EventKindPayment),
				rule("Документ.Оплata", EventKindShipment), // note: different string
				rule("Документ.Оплата", EventKindShipment), // duplicate of first
			},
			wantErr: ErrDuplicateExternalType,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := NewMappingConfig(tt.userID, tt.rules)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && cfg == nil {
				t.Fatal("expected non-nil config")
			}
		})
	}
}

func TestMappingConfig_Resolve(t *testing.T) {
	user := uuid.New()
	cfg, err := NewMappingConfig(user, []MappingRule{
		rule("Документ.ОплатаПокупателя", EventKindPayment),
		rule("Справочник.Контрагенты", EventKindCounterpartyCreated),
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	t.Run("known type resolves", func(t *testing.T) {
		r, ok := cfg.Resolve("Документ.ОплатаПокупателя")
		if !ok {
			t.Fatal("expected match")
		}
		if r.Kind != EventKindPayment {
			t.Fatalf("kind = %q, want payment", r.Kind)
		}
	})

	t.Run("unknown type does not resolve", func(t *testing.T) {
		if _, ok := cfg.Resolve("Документ.Неизвестный"); ok {
			t.Fatal("unknown external type must not resolve")
		}
	})
}
