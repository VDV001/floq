package settings

import "errors"

// ErrAIUnknownProvider — user requested an AI test for a provider name
// the AI tester does not recognise. Wrapped in a typed error so the
// settings handler can translate to a user-facing message via
// aiErrorToUserMessage rather than dumping a raw err.Error() that
// originated in an infrastructure file.
var ErrAIUnknownProvider = errors.New("ai unknown provider")

// ErrAIUnreachable — the AI back-end could not be reached (connection
// refused, non-200, unparseable response). Used for local back-ends
// (Ollama) whose connection test probes liveness rather than generating.
var ErrAIUnreachable = errors.New("ai unreachable")

// ErrAIModelNotFound — the AI back-end is reachable but the configured
// model is not available (e.g. an Ollama model that has not been pulled).
var ErrAIModelNotFound = errors.New("ai model not found")
