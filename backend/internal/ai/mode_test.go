package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModelMode_String(t *testing.T) {
	cases := []struct {
		mode ModelMode
		want string
	}{
		{ModelModeExecute, "execute"},
		{ModelModePlan, "plan"},
		{ModelModeBudget, "budget"},
		{ModelMode(99), "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.mode.String())
		})
	}
}

// modeRecordingProvider records the Mode of the last CompletionRequest
// it received. Used to verify that AIClient methods set the right mode
// per use case.
type modeRecordingProvider struct {
	name         string
	response     string
	recordedMode ModelMode
}

func (m *modeRecordingProvider) Complete(_ context.Context, req CompletionRequest) (string, error) {
	m.recordedMode = req.Mode
	return m.response, nil
}

func (m *modeRecordingProvider) Name() string { return m.name }

func TestAIClient_MethodModeMapping(t *testing.T) {
	jsonResp := `{"identified_need":"x","estimated_budget":"y","deadline":"z","score":1,"score_reason":"r","recommended_action":"a"}`
	cases := []struct {
		name     string
		response string
		want     ModelMode
		call     func(c *AIClient) error
	}{
		{
			name:     "Qualify uses Execute",
			response: jsonResp,
			want:     ModelModeExecute,
			call: func(c *AIClient) error {
				_, err := c.Qualify(context.Background(), "n", "email", "msg")
				return err
			},
		},
		{
			name:     "DraftReply uses Execute",
			response: "draft body",
			want:     ModelModeExecute,
			call: func(c *AIClient) error {
				_, err := c.DraftReply(context.Background(), "n", "co", "email", "msg", "{}")
				return err
			},
		},
		{
			name:     "GenerateFollowup uses Execute",
			response: "followup",
			want:     ModelModeExecute,
			call: func(c *AIClient) error {
				_, err := c.GenerateFollowup(context.Background(), "n", "co", "3", "msg", "reply")
				return err
			},
		},
		{
			name:     "GenerateColdMessage uses Execute",
			response: "cold msg",
			want:     ModelModeExecute,
			call: func(c *AIClient) error {
				_, err := c.GenerateColdMessage(context.Background(), "n", "t", "co", "ctx", "step", "", "src", "")
				return err
			},
		},
		{
			name:     "GenerateTelegramMessage uses Execute",
			response: "tg msg",
			want:     ModelModeExecute,
			call: func(c *AIClient) error {
				_, err := c.GenerateTelegramMessage(context.Background(), "n", "t", "co", "ctx", "step", "", "src", "")
				return err
			},
		},
		{
			name:     "GenerateTelegramReply uses Execute",
			response: "reply",
			want:     ModelModeExecute,
			call: func(c *AIClient) error {
				_, err := c.GenerateTelegramReply(context.Background(), "n", "t", "co", "ctx", "h", "m")
				return err
			},
		},
		{
			name:     "GenerateCallBrief uses Plan",
			response: "brief",
			want:     ModelModePlan,
			call: func(c *AIClient) error {
				_, err := c.GenerateCallBrief(context.Background(), "n", "t", "co", "ctx", "step", "")
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &modeRecordingProvider{response: tc.response}
			c := NewAIClient(p, "", "", "", "", "")
			err := tc.call(c)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, p.recordedMode, "mode for %s", tc.name)
		})
	}
}
