package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProspectStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status ProspectStatus
		want   bool
	}{
		{"new is valid", ProspectStatusNew, true},
		{"in_sequence is valid", ProspectStatusInSequence, true},
		{"converted is valid", ProspectStatusConverted, true},
		{"empty is invalid", ProspectStatus(""), false},
		{"unknown is invalid", ProspectStatus("archived"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("ProspectStatus(%q).IsValid() = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestProspectStatus_String(t *testing.T) {
	tests := []struct {
		status ProspectStatus
		want   string
	}{
		{ProspectStatusNew, "new"},
		{ProspectStatusInSequence, "in_sequence"},
		{ProspectStatusConverted, "converted"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("ProspectStatus.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVerifyStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status VerifyStatus
		want   bool
	}{
		{"not_checked is valid", VerifyStatusNotChecked, true},
		{"valid is valid", VerifyStatusValid, true},
		{"invalid is valid", VerifyStatusInvalid, true},
		{"risky is valid", VerifyStatusRisky, true},
		{"empty is invalid", VerifyStatus(""), false},
		{"unknown is invalid", VerifyStatus("pending"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("VerifyStatus(%q).IsValid() = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestVerifyStatus_String(t *testing.T) {
	tests := []struct {
		status VerifyStatus
		want   string
	}{
		{VerifyStatusNotChecked, "not_checked"},
		{VerifyStatusValid, "valid"},
		{VerifyStatusInvalid, "invalid"},
		{VerifyStatusRisky, "risky"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("VerifyStatus.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProspect_CanLaunchSequence(t *testing.T) {
	tests := []struct {
		name   string
		p      Prospect
		expect bool
	}{
		{"new+valid", Prospect{Status: ProspectStatusNew, VerifyStatus: VerifyStatusValid}, true},
		{"converted", Prospect{Status: ProspectStatusConverted, VerifyStatus: VerifyStatusValid}, false},
		{"opted_out", Prospect{Status: ProspectStatusOptedOut, VerifyStatus: VerifyStatusValid}, false},
		{"in_sequence", Prospect{Status: ProspectStatusInSequence, VerifyStatus: VerifyStatusValid}, false},
		{"invalid verify", Prospect{Status: ProspectStatusNew, VerifyStatus: VerifyStatusInvalid}, false},
		{"not_checked with email", Prospect{Status: ProspectStatusNew, VerifyStatus: VerifyStatusNotChecked, Email: "a@b.com"}, false},
		{"not_checked no email", Prospect{Status: ProspectStatusNew, VerifyStatus: VerifyStatusNotChecked}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.CanLaunchSequence(); got != tt.expect {
				t.Errorf("CanLaunchSequence() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestNewProspect(t *testing.T) {
	userID := uuid.New()
	name := "John Doe"
	company := "Acme Inc"
	title := "CTO"
	email := "john@acme.com"
	source := "linkedin"

	p := NewProspect(userID, name, company, title, email, source)
	require.NotNil(t, p)

	assert.NotEqual(t, uuid.Nil, p.ID)
	assert.Equal(t, userID, p.UserID)
	assert.Equal(t, name, p.Name)
	assert.Equal(t, company, p.Company)
	assert.Equal(t, title, p.Title)
	assert.Equal(t, email, p.Email)
	assert.Equal(t, source, p.Source)
	assert.Equal(t, ProspectStatusNew, p.Status)
	assert.Equal(t, VerifyStatusNotChecked, p.VerifyStatus)
	assert.Equal(t, "{}", p.VerifyDetails)
	assert.False(t, p.CreatedAt.IsZero())
	assert.False(t, p.UpdatedAt.IsZero())
	assert.Equal(t, p.CreatedAt, p.UpdatedAt)
}
