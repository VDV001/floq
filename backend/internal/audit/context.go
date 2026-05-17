package audit

import (
	"context"

	"github.com/google/uuid"

	"github.com/daniil/floq/internal/audit/domain"
)

// CallMeta is the attribution context the audit layer needs to record
// an AI provider call: who issued the request (user_id), which entity
// it was about (lead or prospect, optional), and what business intent
// triggered it (request_type). Call sites attach this to the ctx
// before invoking an AI method; the RecordingProvider decorator pulls
// it back out and stamps the audit row.
//
// Splitting attribution into the context — instead of every provider
// method signature — keeps Provider's interface lean and lets the
// recording wrapper stay transparent to the business code.
type CallMeta struct {
	UserID      uuid.UUID
	LeadID      *uuid.UUID
	ProspectID  *uuid.UUID
	RequestType domain.RequestType
}

type callMetaKey struct{}

// ContextWithCallMeta returns a child ctx carrying meta. Call this at
// the boundary between business code and the AI client so the
// recording layer can attribute the resulting audit row.
func ContextWithCallMeta(ctx context.Context, meta CallMeta) context.Context {
	return context.WithValue(ctx, callMetaKey{}, meta)
}

// CallMetaFromContext extracts meta if a parent set one. The second
// return is false when no meta is attached — the recording layer
// treats this as "skip with warn", not "default everything to zero".
func CallMetaFromContext(ctx context.Context) (CallMeta, bool) {
	v, ok := ctx.Value(callMetaKey{}).(CallMeta)
	return v, ok
}
