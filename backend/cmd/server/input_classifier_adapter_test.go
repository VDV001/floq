package main

import (
	"testing"

	"github.com/daniil/floq/internal/ai/security"
	"github.com/daniil/floq/internal/inbox"
)

func TestMapSecuritySeverity(t *testing.T) {
	cases := []struct {
		in   security.Severity
		want inbox.Severity
	}{
		{security.SeverityInfo, inbox.SeverityInfo},
		{security.SeverityWarn, inbox.SeverityWarn},
		{security.SeverityBlock, inbox.SeverityBlock},
		{security.Severity(99), inbox.SeverityInfo}, // unknown → safe baseline, never escalates
	}
	for _, tc := range cases {
		if got := mapSecuritySeverity(tc.in); got != tc.want {
			t.Errorf("mapSecuritySeverity(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// End-to-end through the real firewall: a benign business inbound must
// classify as Info so legitimate booking replies are never gated. (Block
// mapping is covered exhaustively by TestMapSecuritySeverity; the firewall's
// own block patterns are covered in the security package.)
func TestInboxInputClassifier_BenignMessageIsInfo(t *testing.T) {
	c := newInboxInputClassifier(security.NewInputFirewall())
	benign := c.Classify("Здравствуйте, хотим обсудить внедрение CRM, когда удобно созвониться?")
	if benign != inbox.SeverityInfo {
		t.Errorf("benign business message = %q, want info", benign)
	}
}
