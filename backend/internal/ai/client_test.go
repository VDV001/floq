package ai

import (
	"context"
	"errors"
	"testing"
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
	c := NewAIClient(&mockProvider{name: "openai"}, "", "", "")
	if got := c.ProviderName(); got != "openai" {
		t.Errorf("expected %q, got %q", "openai", got)
	}
}

// --- Qualify ---

func TestQualify_Success(t *testing.T) {
	jsonResp := `{"identified_need":"website","estimated_budget":"100k","deadline":"Q1","score":8,"score_reason":"hot lead","recommended_action":"call"}`
	c := NewAIClient(&mockProvider{response: jsonResp}, "", "", "")

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
	c := NewAIClient(&mockProvider{response: jsonResp}, "", "", "")

	result, err := c.Qualify(context.Background(), "Jane", "telegram", "Need SEO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Score != 5 {
		t.Errorf("expected score 5, got %d", result.Score)
	}
}

func TestQualify_MalformedJSON(t *testing.T) {
	c := NewAIClient(&mockProvider{response: "not json at all"}, "", "", "")

	_, err := c.Qualify(context.Background(), "John", "email", "hello")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestQualify_ProviderError(t *testing.T) {
	c := NewAIClient(&mockProvider{err: errors.New("api down")}, "", "", "")

	_, err := c.Qualify(context.Background(), "John", "email", "hello")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
