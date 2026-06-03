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

func TestParseAuthType(t *testing.T) {
	tests := []struct {
		in      string
		want    AuthType
		wantErr error
	}{
		{"basic", AuthTypeBasic, nil},
		{"token", AuthTypeToken, nil},
		{"", "", ErrInvalidAuthType},
		{"oauth", "", ErrInvalidAuthType},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseAuthType(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewOutboundCredentials(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		authType AuthType
		secret   string
		wantErr  error
		wantURL  string
	}{
		{"valid basic", "https://1c.example/odata", AuthTypeBasic, "dXNlcjpwYXNz", nil, "https://1c.example/odata"},
		{"valid token", "https://1c.example/odata", AuthTypeToken, "tok", nil, "https://1c.example/odata"},
		{"trims trailing slash", "https://1c.example/odata/", AuthTypeBasic, "s", nil, "https://1c.example/odata"},
		{"empty base url", "", AuthTypeBasic, "s", ErrEmptyBaseURL, ""},
		{"blank base url", "   ", AuthTypeBasic, "s", ErrEmptyBaseURL, ""},
		{"invalid auth type", "https://1c.example", AuthType("oauth"), "s", ErrInvalidAuthType, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewOutboundCredentials(tt.baseURL, tt.authType, tt.secret)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if got.BaseURL != tt.wantURL {
				t.Errorf("BaseURL = %q, want %q", got.BaseURL, tt.wantURL)
			}
			if got.AuthType != tt.authType || got.AuthSecret != tt.secret {
				t.Errorf("got %+v", got)
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
