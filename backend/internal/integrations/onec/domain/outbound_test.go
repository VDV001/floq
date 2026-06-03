package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNewCounterpartyDraft(t *testing.T) {
	tests := []struct {
		name              string
		inName            string
		inEmail           string
		inCompany         string
		wantErr           error
		wantName          string
		wantEmail         string
		wantCompany       string
	}{
		{"name and email", "Иван", "iv@ex.ru", "ООО Ромашка", nil, "Иван", "iv@ex.ru", "ООО Ромашка"},
		{"email only", "", "iv@ex.ru", "", nil, "", "iv@ex.ru", ""},
		{"name only", "Иван", "", "", nil, "Иван", "", ""},
		{"trims whitespace", "  Иван  ", "  iv@ex.ru ", " ООО ", nil, "Иван", "iv@ex.ru", "ООО"},
		{"both blank", "   ", "", "Company", ErrEmptyCounterparty, "", "", ""},
		{"all empty", "", "", "", ErrEmptyCounterparty, "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewCounterpartyDraft(tt.inName, tt.inEmail, tt.inCompany)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if got.Name != tt.wantName || got.Email != tt.wantEmail || got.Company != tt.wantCompany {
				t.Fatalf("draft = %+v, want name=%q email=%q company=%q", got, tt.wantName, tt.wantEmail, tt.wantCompany)
			}
		})
	}
}

func TestSyncStatus_IsValid(t *testing.T) {
	valid := []SyncStatus{SyncStatusReceived, SyncStatusProcessed, SyncStatusError}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("SyncStatus(%q).IsValid() = false, want true", s)
		}
	}
	invalid := []SyncStatus{"", "pending", "sent", "done"}
	for _, s := range invalid {
		if s.IsValid() {
			t.Errorf("SyncStatus(%q).IsValid() = true, want false", s)
		}
	}
}

func TestNewOutboundSyncRecord(t *testing.T) {
	user := uuid.New()
	tests := []struct {
		name         string
		userID       uuid.UUID
		externalID   string
		externalType string
		kind         EventKind
		status       SyncStatus
		wantErr      error
	}{
		{"valid processed", user, "prospect:iv@ex.ru", "counterparty", EventKindCounterpartyCreated, SyncStatusProcessed, nil},
		{"valid error", user, "prospect:iv@ex.ru", "counterparty", EventKindCounterpartyCreated, SyncStatusError, nil},
		{"nil user", uuid.Nil, "x", "counterparty", EventKindCounterpartyCreated, SyncStatusProcessed, ErrNilUser},
		{"empty external id", user, "", "counterparty", EventKindCounterpartyCreated, SyncStatusProcessed, ErrEmptyExternalID},
		{"empty external type", user, "x", "", EventKindCounterpartyCreated, SyncStatusProcessed, ErrEmptyExternalType},
		{"invalid kind", user, "x", "counterparty", EventKind("bogus"), SyncStatusProcessed, ErrInvalidEventKind},
		{"invalid status", user, "x", "counterparty", EventKindCounterpartyCreated, SyncStatus("bogus"), ErrInvalidSyncStatus},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewOutboundSyncRecord(tt.userID, tt.externalID, tt.externalType, tt.kind, tt.status)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if got.Direction != DirectionOutbound {
				t.Errorf("Direction = %q, want outbound", got.Direction)
			}
			if got.Status != tt.status {
				t.Errorf("Status = %q, want %q", got.Status, tt.status)
			}
			if got.ID == uuid.Nil {
				t.Error("ID not generated")
			}
		})
	}
}
