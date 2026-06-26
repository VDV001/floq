package ai

import (
	"context"
	"errors"
	"fmt"
)

// ErrVisionUnsupported is returned by AIClient.AnalyzeImage when the
// underlying provider does not implement VisionProvider. Callers
// should treat this as a soft-fail: skip the attachment and let the
// lead through.
var ErrVisionUnsupported = errors.New("ai provider does not support vision")

// VisionProvider is an optional capability on top of Provider:
// implementations expose multimodal analysis of an image plus a text
// prompt. Only providers that actually support this (OpenAI's gpt-4o /
// gpt-4o-mini, Anthropic's Claude vision models) should implement it.
// Ollama and any text-only providers should NOT implement it; the
// AIClient detects this via type assertion and degrades gracefully.
type VisionProvider interface {
	AnalyzeImage(ctx context.Context, imageData []byte, mimeType, prompt string) (*CompletionResult, error)
}

// AnalyzeImage sends imageData (raw bytes, e.g. PNG or JPEG) together
// with prompt to the provider's vision endpoint and returns the
// transcribed / described text. Returns ErrVisionUnsupported when the
// active provider is text-only; the caller's typical response is to
// skip the attachment with a warn-level log entry.
func (c *AIClient) AnalyzeImage(ctx context.Context, imageData []byte, mimeType, prompt string) (string, error) {
	vp, ok := c.provider.(VisionProvider)
	if !ok {
		return "", ErrVisionUnsupported
	}
	result, err := vp.AnalyzeImage(ctx, imageData, mimeType, prompt)
	if err != nil {
		return "", fmt.Errorf("analyze image: %w", err)
	}
	return result.Text, nil
}
