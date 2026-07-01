package providers

import "errors"

// Ollama health-check sentinels. CheckHealth talks to the native
// GET /api/tags endpoint (instant, does not load a model) instead of a
// full generation, so a connection test no longer trips the cold-start
// timeout. The composition root maps these to the settings-handler's
// vocabulary via errors.Is, which in turn maps to user-facing copy.
var (
	// ErrOllamaUnreachable — the Ollama server could not be reached, or
	// answered /api/tags with a non-200 / unparseable body.
	ErrOllamaUnreachable = errors.New("ollama unreachable")

	// ErrOllamaModelNotFound — Ollama is reachable but the configured
	// model is not among the locally-pulled models.
	ErrOllamaModelNotFound = errors.New("ollama model not found")
)
