package enrichment

import (
	"context"

	"github.com/google/uuid"
)

// subjectUserKey carries the user a background enrichment is attributed to,
// from the usecase down to the Completer adapter — without the enrichment
// context importing internal/audit (cross-module rule). The composition-root
// adapter reads it to build the audit CallMeta, so an LLM extraction issued by
// the cron worker is cost-attributed to the right user.
type subjectUserKey struct{}

// WithSubjectUser tags ctx with the user the enrichment is attributed to. A
// Nil user is treated as no attribution (not stored), so SubjectUserFromContext
// never reports a Nil user as present — the audit CallMeta it would build is
// rejected by NewEntry.
func WithSubjectUser(ctx context.Context, userID uuid.UUID) context.Context {
	if userID == uuid.Nil {
		return ctx
	}
	return context.WithValue(ctx, subjectUserKey{}, userID)
}

// SubjectUserFromContext returns the attributed user, if one was set.
func SubjectUserFromContext(ctx context.Context) (uuid.UUID, bool) {
	uid, ok := ctx.Value(subjectUserKey{}).(uuid.UUID)
	return uid, ok
}
