package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// ErrInvalidUnsubscribeToken is returned for any token that fails to verify:
// malformed, tampered, or signed with a different secret. It is deliberately
// the single error for every failure mode so the unsubscribe endpoint cannot
// be used as an oracle that distinguishes "wrong signature" from "bad format".
var ErrInvalidUnsubscribeToken = errors.New("unsubscribe: invalid token")

// unsubscribeTokenContext is a domain-separation prefix mixed into the MAC so
// these signatures can never collide with another use of the same secret as an
// HMAC key. Bump the version suffix if the token layout ever changes.
const unsubscribeTokenContext = "floq-unsubscribe-v1"

// signUnsubscribe computes the MAC over the prospect ID bytes under secret,
// domain-separated by unsubscribeTokenContext.
func signUnsubscribe(payload []byte, secret string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(unsubscribeTokenContext))
	mac.Write([]byte{0})
	mac.Write(payload)
	return mac.Sum(nil)
}

// SignUnsubscribeToken returns an unguessable, URL-safe token that authorizes
// withdrawal of consent for prospectID. The token is an HMAC of the prospect
// ID under secret, so it cannot be forged or enumerated without the key —
// unlike the tracking pixel, which exposes a raw message ID.
func SignUnsubscribeToken(prospectID uuid.UUID, secret string) string {
	payload := prospectID[:]
	sig := signUnsubscribe(payload, secret)
	enc := base64.RawURLEncoding
	return enc.EncodeToString(payload) + "." + enc.EncodeToString(sig)
}

// ParseUnsubscribeToken verifies token against secret and returns the prospect
// ID it authorizes. Returns ErrInvalidUnsubscribeToken on any failure. The
// signature comparison is constant-time.
func ParseUnsubscribeToken(token, secret string) (uuid.UUID, error) {
	payloadPart, sigPart, ok := strings.Cut(token, ".")
	if !ok || strings.Contains(sigPart, ".") {
		return uuid.Nil, ErrInvalidUnsubscribeToken
	}
	enc := base64.RawURLEncoding
	payload, err := enc.DecodeString(payloadPart)
	if err != nil {
		return uuid.Nil, ErrInvalidUnsubscribeToken
	}
	sig, err := enc.DecodeString(sigPart)
	if err != nil {
		return uuid.Nil, ErrInvalidUnsubscribeToken
	}
	if !hmac.Equal(sig, signUnsubscribe(payload, secret)) {
		return uuid.Nil, ErrInvalidUnsubscribeToken
	}
	id, err := uuid.FromBytes(payload)
	if err != nil {
		return uuid.Nil, ErrInvalidUnsubscribeToken
	}
	return id, nil
}
