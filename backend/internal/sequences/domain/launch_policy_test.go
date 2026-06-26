package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// InitialOutboundStatus is the launch-time HITL decision: a message skips
// human review (starts Approved) ONLY when autopilot is on AND the sequence
// does not require approval. The per-sequence approval gate overrides
// autopilot, so a cautious sequence is always reviewed before send.
func TestInitialOutboundStatus(t *testing.T) {
	tests := []struct {
		name             string
		autopilotEnabled bool
		requireApproval  bool
		want             OutboundStatus
	}{
		{"autopilot off, no gate → draft (manual approval, unchanged default)", false, false, OutboundStatusDraft},
		{"autopilot on, no gate → approved (auto-send)", true, false, OutboundStatusApproved},
		{"autopilot off, gate on → draft", false, true, OutboundStatusDraft},
		{"autopilot on, gate on → draft (gate overrides autopilot)", true, true, OutboundStatusDraft},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, InitialOutboundStatus(tt.autopilotEnabled, tt.requireApproval))
		})
	}
}

func TestNewSequence_DefaultsRequireApprovalFalse(t *testing.T) {
	s, err := NewSequence(uuid.New(), "Outreach")
	require.NoError(t, err)
	assert.False(t, s.RequireApproval, "new sequences keep the prior behaviour; the approval gate is opt-in")
}
