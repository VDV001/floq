package domain

import "testing"

func TestLeadStatus_IsValid(t *testing.T) {
	valid := []LeadStatus{StatusNew, StatusQualified, StatusClosed, StatusWon}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("expected %q to be valid", s)
		}
	}

	invalid := []LeadStatus{"", "pending", "lost", "archived"}
	for _, s := range invalid {
		if s.IsValid() {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func TestLeadStatus_String(t *testing.T) {
	tests := []struct {
		status LeadStatus
		want   string
	}{
		{StatusNew, "new"},
		{StatusQualified, "qualified"},
		{StatusClosed, "closed"},
		{StatusWon, "won"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("LeadStatus(%q).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestLeadStatus_CanTransitionTo(t *testing.T) {
	allowed := []struct {
		from, to LeadStatus
	}{
		{StatusNew, StatusQualified},
		{StatusNew, StatusClosed},
		{StatusQualified, StatusClosed},
		{StatusQualified, StatusWon},
		{StatusWon, StatusClosed},
	}
	for _, tt := range allowed {
		if !tt.from.CanTransitionTo(tt.to) {
			t.Errorf("expected transition %s -> %s to be allowed", tt.from, tt.to)
		}
	}

	disallowed := []struct {
		from, to LeadStatus
	}{
		{StatusNew, StatusWon},
		{StatusNew, StatusNew},
		{StatusQualified, StatusNew},
		{StatusQualified, StatusQualified},
		{StatusClosed, StatusNew},
		{StatusClosed, StatusQualified},
		{StatusClosed, StatusWon},
		{StatusClosed, StatusClosed},
		{StatusWon, StatusNew},
		{StatusWon, StatusQualified},
		{StatusWon, StatusWon},
	}
	for _, tt := range disallowed {
		if tt.from.CanTransitionTo(tt.to) {
			t.Errorf("expected transition %s -> %s to be disallowed", tt.from, tt.to)
		}
	}
}

func TestChannel_IsValid(t *testing.T) {
	valid := []Channel{ChannelTelegram, ChannelEmail}
	for _, c := range valid {
		if !c.IsValid() {
			t.Errorf("expected %q to be valid", c)
		}
	}

	invalid := []Channel{"", "sms", "whatsapp", "slack"}
	for _, c := range invalid {
		if c.IsValid() {
			t.Errorf("expected %q to be invalid", c)
		}
	}
}
