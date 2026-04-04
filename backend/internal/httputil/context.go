package httputil

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// contextKey is an unexported type for context keys in this package,
// preventing collisions with keys defined in other packages.
type contextKey string

const userIDKey contextKey = "user_id"

// WithUserID returns a new context with the given user ID stored.
func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// UserIDFromContext extracts the user ID from the context.
// Returns uuid.Nil and false if not present.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}

// ParseIDParam extracts and parses a UUID URL parameter from a chi route.
func ParseIDParam(r *http.Request, param string) (uuid.UUID, error) {
	raw := chi.URLParam(r, param)
	if raw == "" {
		return uuid.Nil, fmt.Errorf("missing URL param %q", param)
	}
	return uuid.Parse(raw)
}
