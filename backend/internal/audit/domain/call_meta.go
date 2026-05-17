package domain

import (
	"context"

	"github.com/google/uuid"
)

// CallMeta is the attribution payload every AI provider call must
// carry so the recording layer can stamp the resulting audit_log row.
// It lives in the domain package so consumer code outside the audit
// bounded context (leads, sequences, inbox, reminders, chat) can
// import this DTO surface without pulling in the recording machinery
// — the same way a use case is allowed to import "uuid" or "time"
// without breaking layering.
//
// LeadID and ProspectID are optional: one is set per call, both nil is
// also valid (e.g. chat-assist has no attached entity).
type CallMeta struct {
	UserID      uuid.UUID
	LeadID      *uuid.UUID
	ProspectID  *uuid.UUID
	RequestType RequestType
}

type callMetaKey struct{}

// ContextWithCallMeta returns a child ctx carrying meta. Call this at
// the boundary between business code and the AI client.
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

// WithRequestType returns ctx with the same attribution but the
// RequestType field overridden. Used by the AI style-check pass to
// re-tag its inner provider.Complete call as style_check while keeping
// the parent's user/lead/prospect attribution. When no meta is
// attached, the original ctx is returned unchanged.
func WithRequestType(ctx context.Context, rt RequestType) context.Context {
	meta, ok := CallMetaFromContext(ctx)
	if !ok {
		return ctx
	}
	meta.RequestType = rt
	return ContextWithCallMeta(ctx, meta)
}
