package settings

import "errors"

// ErrAIUnknownProvider — user requested an AI test for a provider name
// the AI tester does not recognise. Wrapped in a typed error so the
// settings handler can translate to a user-facing message via
// aiErrorToUserMessage rather than dumping a raw err.Error() that
// originated in an infrastructure file.
var ErrAIUnknownProvider = errors.New("ai unknown provider")
