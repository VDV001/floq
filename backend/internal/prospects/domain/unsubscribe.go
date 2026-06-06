package domain

import (
	"errors"

	"github.com/google/uuid"
)

// ErrInvalidUnsubscribeToken is returned for any token that fails to verify:
// malformed, tampered, or signed with a different secret. It is deliberately
// the single error for every failure mode so the unsubscribe endpoint cannot
// be used as an oracle that distinguishes "wrong signature" from "bad format".
var ErrInvalidUnsubscribeToken = errors.New("unsubscribe: invalid token")

// SignUnsubscribeToken returns an unguessable, URL-safe token that authorizes
// withdrawal of consent for prospectID. The token is an HMAC of the prospect
// ID under secret, so it cannot be forged or enumerated without the key —
// unlike the tracking pixel, which exposes a raw message ID.
func SignUnsubscribeToken(prospectID uuid.UUID, secret string) string {
	panic("not implemented")
}

// ParseUnsubscribeToken verifies token against secret and returns the prospect
// ID it authorizes. Returns ErrInvalidUnsubscribeToken on any failure. The
// signature comparison is constant-time.
func ParseUnsubscribeToken(token, secret string) (uuid.UUID, error) {
	panic("not implemented")
}
