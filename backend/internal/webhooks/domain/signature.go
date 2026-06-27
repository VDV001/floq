package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// signatureContext domain-separates the webhook MAC so an endpoint secret used
// here can never collide with the same string used as a key elsewhere. Bump the
// version suffix if the signing scheme ever changes.
const signatureContext = "floq-webhook-v1"

// signatureHeader is the HTTP header carrying the payload signature. Receivers
// recompute HMAC-SHA256 over the raw body under the shared secret to verify
// authenticity — the same scheme GitHub/Stripe webhooks use.
const SignatureHeader = "X-Floq-Signature"

// EventIDHeader carries the delivery's stable id so a receiver can dedup retries
// of the same delivery (at-least-once → effectively-once on the receiver side).
// Unsigned by design: authenticity is the signature's job; this is only an
// idempotency key, mirroring GitHub's X-GitHub-Delivery.
const EventIDHeader = "X-Floq-Event-Id"

// computeMAC returns the raw HMAC-SHA256 of payload under secret, domain-
// separated by signatureContext (mirrors the unsubscribe-token signing in the
// prospects context).
func computeMAC(payload []byte, secret string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signatureContext))
	mac.Write([]byte{0})
	mac.Write(payload)
	return mac.Sum(nil)
}

// SignPayload returns the value for the X-Floq-Signature header: the hex-encoded
// HMAC-SHA256 of the raw payload, prefixed "sha256=" so the algorithm is
// explicit and future-proof.
func SignPayload(payload []byte, secret string) string {
	return "sha256=" + hex.EncodeToString(computeMAC(payload, secret))
}

// VerifyPayloadSignature reports whether sig is a valid signature for payload
// under secret. The comparison is constant-time. Provided for tests and any
// future inbound verification; the delivery path only signs.
func VerifyPayloadSignature(payload []byte, secret, sig string) bool {
	return hmac.Equal([]byte(sig), []byte(SignPayload(payload, secret)))
}
