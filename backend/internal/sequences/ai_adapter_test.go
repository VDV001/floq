package sequences

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAIProvider struct {
	resp string
}

func (f *fakeAIProvider) Name() string { return "test" }
func (f *fakeAIProvider) Complete(_ context.Context, _ ai.CompletionRequest) (string, error) {
	return f.resp, nil
}

func TestAIMessageGeneratorAdapter_GenerateColdMessage(t *testing.T) {
	client := ai.NewAIClient(&fakeAIProvider{resp: "cold msg"}, "", "", "", "", "")
	adapter := NewAIMessageGeneratorAdapter(client)

	msg, err := adapter.GenerateColdMessage(context.Background(), "Ivan", "CEO", "Acme", "", "", "", "", "")
	require.NoError(t, err)
	assert.Equal(t, "cold msg", msg)
}

func TestAIMessageGeneratorAdapter_GenerateTelegramMessage(t *testing.T) {
	client := ai.NewAIClient(&fakeAIProvider{resp: "tg msg"}, "", "", "", "", "")
	adapter := NewAIMessageGeneratorAdapter(client)

	msg, err := adapter.GenerateTelegramMessage(context.Background(), "Ivan", "CEO", "Acme", "", "", "", "", "")
	require.NoError(t, err)
	assert.Equal(t, "tg msg", msg)
}

func TestAIMessageGeneratorAdapter_GenerateCallBrief(t *testing.T) {
	client := ai.NewAIClient(&fakeAIProvider{resp: "call brief"}, "", "", "", "", "")
	adapter := NewAIMessageGeneratorAdapter(client)

	msg, err := adapter.GenerateCallBrief(context.Background(), "Ivan", "CEO", "Acme", "", "", "")
	require.NoError(t, err)
	assert.Equal(t, "call brief", msg)
}
