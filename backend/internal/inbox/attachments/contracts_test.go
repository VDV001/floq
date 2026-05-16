package attachments_test

import (
	"github.com/daniil/floq/internal/ai"
	"github.com/daniil/floq/internal/inbox/attachments"
)

// Compile-time assertion that *ai.AIClient satisfies the consumer-side
// VisionClient interface this package declares. Two interfaces with
// identical signatures live in two packages (attachments owns the
// consumer side, ai owns the provider side); without this check a
// signature drift in either file would only surface at composition
// time in cmd/server. Keeping it in a _test.go file keeps the
// production import graph free of the ai → attachments edge.
var _ attachments.VisionClient = (*ai.AIClient)(nil)
