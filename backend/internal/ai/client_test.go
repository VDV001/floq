package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- extractJSON ---

func TestExtractJSON_CleanJSON(t *testing.T) {
	input := `{"score":5,"reason":"good"}`
	got := extractJSON(input)
	if got != input {
		t.Errorf("expected %q, got %q", input, got)
	}
}

func TestExtractJSON_MarkdownFences(t *testing.T) {
	input := "```json\n{\"score\":5}\n```"
	want := `{"score":5}`
	got := extractJSON(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestExtractJSON_MarkdownFencesNoLang(t *testing.T) {
	input := "```\n{\"score\":5}\n```"
	want := `{"score":5}`
	got := extractJSON(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestExtractJSON_MixedText(t *testing.T) {
	input := `Here is the result: {"score":5,"reason":"good"} hope that helps!`
	want := `{"score":5,"reason":"good"}`
	got := extractJSON(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestExtractJSON_WhitespaceAround(t *testing.T) {
	input := "  \n  {\"a\":1}  \n  "
	want := `{"a":1}`
	got := extractJSON(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// --- resolveSystemPrompt ---

func TestResolveSystemPrompt_ReplacesBookingLink(t *testing.T) {
	c := &AIClient{bookingLink: "https://cal.com/test"}
	input := "Book a call at {{booking_link}} now"
	want := "Book a call at https://cal.com/test now"
	got := c.resolveSystemPrompt(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolveSystemPrompt_NoPlaceholder(t *testing.T) {
	c := &AIClient{bookingLink: "https://cal.com/test"}
	input := "No placeholder here"
	got := c.resolveSystemPrompt(input)
	if got != input {
		t.Errorf("expected %q, got %q", input, got)
	}
}

// --- resolveSenderVars ---

func TestResolveSenderVars_ReplacesBoth(t *testing.T) {
	c := &AIClient{senderName: "Alice", senderCompany: "Acme"}
	input := "Hi, I'm {{sender_name}} from {{sender_company}}"
	want := "Hi, I'm Alice from Acme"
	got := c.resolveSenderVars(input)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolveSenderVars_NoPlaceholders(t *testing.T) {
	c := &AIClient{senderName: "Alice", senderCompany: "Acme"}
	input := "plain text"
	got := c.resolveSenderVars(input)
	if got != input {
		t.Errorf("expected %q, got %q", input, got)
	}
}

// --- mockProvider ---

type mockProvider struct {
	name     string
	response string
	err      error
}

func (m *mockProvider) Complete(_ context.Context, _ CompletionRequest) (string, error) {
	return m.response, m.err
}

func (m *mockProvider) Name() string {
	return m.name
}

// --- ProviderName ---

func TestProviderName(t *testing.T) {
	c := NewAIClient(&mockProvider{name: "openai"}, "", "", "", "", "")
	if got := c.ProviderName(); got != "openai" {
		t.Errorf("expected %q, got %q", "openai", got)
	}
}

// --- Qualify ---

func TestQualify_Success(t *testing.T) {
	jsonResp := `{"identified_need":"website","estimated_budget":"100k","deadline":"Q1","score":8,"score_reason":"hot lead","recommended_action":"call"}`
	c := NewAIClient(&mockProvider{response: jsonResp}, "", "", "", "", "")

	result, err := c.Qualify(context.Background(), "John", "email", "I need a website")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Score != 8 {
		t.Errorf("expected score 8, got %d", result.Score)
	}
	if result.IdentifiedNeed != "website" {
		t.Errorf("expected identified_need %q, got %q", "website", result.IdentifiedNeed)
	}
	if result.RecommendedAction != "call" {
		t.Errorf("expected recommended_action %q, got %q", "call", result.RecommendedAction)
	}
}

func TestQualify_WithMarkdownFences(t *testing.T) {
	jsonResp := "```json\n{\"identified_need\":\"seo\",\"estimated_budget\":\"50k\",\"deadline\":\"Q2\",\"score\":5,\"score_reason\":\"medium\",\"recommended_action\":\"email\"}\n```"
	c := NewAIClient(&mockProvider{response: jsonResp}, "", "", "", "", "")

	result, err := c.Qualify(context.Background(), "Jane", "telegram", "Need SEO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Score != 5 {
		t.Errorf("expected score 5, got %d", result.Score)
	}
}

func TestQualify_MalformedJSON(t *testing.T) {
	c := NewAIClient(&mockProvider{response: "not json at all"}, "", "", "", "", "")

	_, err := c.Qualify(context.Background(), "John", "email", "hello")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestQualify_ProviderError(t *testing.T) {
	c := NewAIClient(&mockProvider{err: errors.New("api down")}, "", "", "", "", "")

	_, err := c.Qualify(context.Background(), "John", "email", "hello")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- DraftReply ---

func TestDraftReply_Success(t *testing.T) {
	c := NewAIClient(&mockProvider{response: "Hello John, thanks for reaching out!"}, "https://cal.com/test", "", "", "", "")

	reply, err := c.DraftReply(context.Background(), "John", "Acme", "email", "Need a website", `{"score":8}`)
	assert.NoError(t, err)
	assert.Equal(t, "Hello John, thanks for reaching out!", reply)
}

func TestDraftReply_ProviderError(t *testing.T) {
	c := NewAIClient(&mockProvider{err: errors.New("timeout")}, "", "", "", "", "")

	_, err := c.DraftReply(context.Background(), "John", "Acme", "email", "hello", "{}")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ai draft reply")
}

func TestDraftReply_EmptyResponse(t *testing.T) {
	c := NewAIClient(&mockProvider{response: ""}, "", "", "", "", "")

	reply, err := c.DraftReply(context.Background(), "John", "Acme", "email", "hello", "{}")
	assert.NoError(t, err)
	assert.Equal(t, "", reply)
}

// --- GenerateFollowup ---

func TestGenerateFollowup_Success(t *testing.T) {
	c := NewAIClient(&mockProvider{response: "Just checking in..."}, "", "", "", "", "")

	reply, err := c.GenerateFollowup(context.Background(), "Jane", "Corp", "3", "Need help", "We can help")
	assert.NoError(t, err)
	assert.Equal(t, "Just checking in...", reply)
}

func TestGenerateFollowup_ProviderError(t *testing.T) {
	c := NewAIClient(&mockProvider{err: errors.New("rate limit")}, "", "", "", "", "")

	_, err := c.GenerateFollowup(context.Background(), "Jane", "Corp", "3", "msg", "reply")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ai generate followup")
}

// --- GenerateColdMessage ---

func TestGenerateColdMessage_Success(t *testing.T) {
	c := NewAIClient(
		&mockProvider{response: "Hi, I'm reaching out about..."},
		"https://cal.com/demo",
		"Alice",
		"Floq",
		"+79001234567",
		"https://floq.app",
	)

	msg, err := c.GenerateColdMessage(context.Background(), "Ivan", "CEO", "TechCorp", "context", "step1", "", "2GIS", "")
	assert.NoError(t, err)
	assert.Equal(t, "Hi, I'm reaching out about...", msg)
}

func TestGenerateColdMessage_WithPreviousMessage(t *testing.T) {
	c := NewAIClient(&mockProvider{response: "follow-up cold"}, "", "Bob", "Co", "", "")

	msg, err := c.GenerateColdMessage(context.Background(), "Ivan", "CTO", "Corp", "ctx", "step2", "prev message", "CSV", "")
	assert.NoError(t, err)
	assert.Equal(t, "follow-up cold", msg)
}

func TestGenerateColdMessage_ProviderError(t *testing.T) {
	c := NewAIClient(&mockProvider{err: errors.New("fail")}, "", "", "", "", "")

	_, err := c.GenerateColdMessage(context.Background(), "N", "T", "C", "ctx", "s", "", "src", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ai cold message")
}

func TestGenerateColdMessage_WithFeedback(t *testing.T) {
	p := &mockProvider{response: "cold msg"}
	c := NewAIClient(p, "", "Alice", "Floq", "", "")

	msg, err := c.GenerateColdMessage(context.Background(), "Ivan", "CEO", "Corp", "ctx", "step", "", "src", "feedback examples here")
	assert.NoError(t, err)
	assert.Equal(t, "cold msg", msg)
}

// --- GenerateTelegramMessage ---

func TestGenerateTelegramMessage_Success(t *testing.T) {
	c := NewAIClient(
		&mockProvider{response: "Telegram outreach msg"},
		"https://cal.com/demo",
		"Alice",
		"Floq",
		"+7900",
		"https://floq.app",
	)

	msg, err := c.GenerateTelegramMessage(context.Background(), "Ivan", "CEO", "Corp", "ctx", "step1", "", "2GIS", "")
	assert.NoError(t, err)
	assert.Equal(t, "Telegram outreach msg", msg)
}

func TestGenerateTelegramMessage_WithPreviousMessage(t *testing.T) {
	c := NewAIClient(&mockProvider{response: "tg follow"}, "", "Bob", "Co", "", "")

	msg, err := c.GenerateTelegramMessage(context.Background(), "Ivan", "CTO", "Corp", "ctx", "step2", "prev", "CSV", "")
	assert.NoError(t, err)
	assert.Equal(t, "tg follow", msg)
}

func TestGenerateTelegramMessage_ProviderError(t *testing.T) {
	c := NewAIClient(&mockProvider{err: errors.New("fail")}, "", "", "", "", "")

	_, err := c.GenerateTelegramMessage(context.Background(), "N", "T", "C", "ctx", "s", "", "src", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ai telegram message")
}

// --- GenerateTelegramReply ---

func TestGenerateTelegramReply_Success(t *testing.T) {
	c := NewAIClient(&mockProvider{response: "Thanks for your reply!"}, "https://cal.com/demo", "Alice", "Floq", "", "")

	result, err := c.GenerateTelegramReply(context.Background(), "Ivan", "CEO", "Corp", "ctx", "history", "last msg")
	assert.NoError(t, err)
	assert.Equal(t, "Thanks for your reply!", result.Text)
	assert.False(t, result.NeedsEscalation)
	assert.Empty(t, result.EscalationNote)
}

func TestGenerateTelegramReply_Escalation(t *testing.T) {
	c := NewAIClient(&mockProvider{response: "[ТРЕБУЕТСЯ МЕНЕДЖЕР] Complex pricing question"}, "", "Alice", "Floq", "", "")

	result, err := c.GenerateTelegramReply(context.Background(), "Ivan", "CEO", "Corp", "ctx", "history", "last msg")
	assert.NoError(t, err)
	assert.True(t, result.NeedsEscalation)
	assert.Equal(t, "Complex pricing question", result.EscalationNote)
}

func TestGenerateTelegramReply_ProviderError(t *testing.T) {
	c := NewAIClient(&mockProvider{err: errors.New("fail")}, "", "", "", "", "")

	_, err := c.GenerateTelegramReply(context.Background(), "N", "T", "C", "ctx", "h", "m")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ai telegram reply")
}

// --- GenerateCallBrief ---

func TestGenerateCallBrief_Success(t *testing.T) {
	c := NewAIClient(&mockProvider{response: "Call brief content"}, "", "", "", "", "")

	brief, err := c.GenerateCallBrief(context.Background(), "Ivan", "CEO", "Corp", "ctx", "step1", "")
	assert.NoError(t, err)
	assert.Equal(t, "Call brief content", brief)
}

func TestGenerateCallBrief_WithPreviousMessage(t *testing.T) {
	c := NewAIClient(&mockProvider{response: "Brief with context"}, "", "", "", "", "")

	brief, err := c.GenerateCallBrief(context.Background(), "Ivan", "CEO", "Corp", "ctx", "step1", "prev msg")
	assert.NoError(t, err)
	assert.Equal(t, "Brief with context", brief)
}

func TestGenerateCallBrief_ProviderError(t *testing.T) {
	c := NewAIClient(&mockProvider{err: errors.New("fail")}, "", "", "", "", "")

	_, err := c.GenerateCallBrief(context.Background(), "N", "T", "C", "ctx", "s", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ai call brief")
}

// --- Complete ---

func TestComplete_Success(t *testing.T) {
	c := NewAIClient(&mockProvider{response: "raw completion"}, "", "", "", "", "")

	resp, err := c.Complete(context.Background(), CompletionRequest{
		Messages:  []Message{{Role: "user", Content: "hello"}},
		MaxTokens: 100,
	})
	assert.NoError(t, err)
	assert.Equal(t, "raw completion", resp)
}

func TestComplete_ProviderError(t *testing.T) {
	c := NewAIClient(&mockProvider{err: errors.New("fail")}, "", "", "", "", "")

	_, err := c.Complete(context.Background(), CompletionRequest{
		Messages:  []Message{{Role: "user", Content: "hello"}},
		MaxTokens: 100,
	})
	assert.Error(t, err)
}

// --- NewAIClient ---

func TestNewAIClient_AllFields(t *testing.T) {
	p := &mockProvider{name: "test"}
	c := NewAIClient(p, "link", "name", "company", "phone", "website")

	assert.Equal(t, "test", c.ProviderName())
	assert.Equal(t, "link", c.bookingLink)
	assert.Equal(t, "name", c.senderName)
	assert.Equal(t, "company", c.senderCompany)
	assert.Equal(t, "phone", c.senderPhone)
	assert.Equal(t, "website", c.senderWebsite)
}

// --- resolveSenderVars with phone and website ---

func TestResolveSenderVars_AllPlaceholders(t *testing.T) {
	c := &AIClient{senderName: "Alice", senderCompany: "Acme", senderPhone: "+7900", senderWebsite: "https://acme.com"}
	input := "{{sender_name}} from {{sender_company}}, call {{sender_phone}}, visit {{sender_website}}"
	want := "Alice from Acme, call +7900, visit https://acme.com"
	got := c.resolveSenderVars(input)
	assert.Equal(t, want, got)
}

// --- extractJSON edge cases ---

func TestExtractJSON_EmptyString(t *testing.T) {
	got := extractJSON("")
	assert.Equal(t, "", got)
}

func TestExtractJSON_OnlyFences(t *testing.T) {
	got := extractJSON("```\n```")
	assert.Equal(t, "", got)
}

func TestExtractJSON_NestedBraces(t *testing.T) {
	input := `{"a":{"b":1}}`
	got := extractJSON(input)
	assert.Equal(t, `{"a":{"b":1}}`, got)
}
