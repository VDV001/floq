package leads

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/leads/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeProvider struct {
	name string
	resp string
	err  error
}

func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) Complete(_ context.Context, _ ai.CompletionRequest) (string, error) {
	return f.resp, f.err
}

func TestAIAdapter_ProviderName(t *testing.T) {
	client := ai.NewAIClient(&fakeProvider{name: "test-provider"}, "", "", "", "", "")
	adapter := NewAIAdapter(client)
	assert.Equal(t, "test-provider", adapter.ProviderName())
}

func TestAIAdapter_Qualify(t *testing.T) {
	resp := `{"identified_need":"CRM","estimated_budget":"100k","deadline":"1 month","score":80,"score_reason":"high intent","recommended_action":"call"}`
	client := ai.NewAIClient(&fakeProvider{name: "test", resp: resp}, "", "", "", "", "")
	adapter := NewAIAdapter(client)

	q, err := adapter.Qualify(context.Background(), "Ivan", domain.ChannelTelegram, "Need a CRM")
	require.NoError(t, err)
	assert.Equal(t, "CRM", q.IdentifiedNeed)
	assert.Equal(t, 80, q.Score)
	assert.Equal(t, "test", q.ProviderUsed)
}

func TestAIAdapter_Qualify_Error(t *testing.T) {
	client := ai.NewAIClient(&fakeProvider{name: "test", resp: "not json"}, "", "", "", "", "")
	adapter := NewAIAdapter(client)

	_, err := adapter.Qualify(context.Background(), "Ivan", domain.ChannelTelegram, "hello")
	assert.Error(t, err)
}

func TestAIAdapter_DraftReply(t *testing.T) {
	client := ai.NewAIClient(&fakeProvider{name: "test", resp: "Draft reply text"}, "", "", "", "", "")
	adapter := NewAIAdapter(client)

	reply, err := adapter.DraftReply(context.Background(), "Ivan", "Hello")
	require.NoError(t, err)
	assert.Equal(t, "Draft reply text", reply)
}

func TestAIAdapter_GenerateFollowup(t *testing.T) {
	client := ai.NewAIClient(&fakeProvider{name: "test", resp: "Followup text"}, "", "", "", "", "")
	adapter := NewAIAdapter(client)

	msg, err := adapter.GenerateFollowup(context.Background(), "Ivan", "Acme", 3)
	require.NoError(t, err)
	assert.Equal(t, "Followup text", msg)
}
