package domain

import "testing"

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
