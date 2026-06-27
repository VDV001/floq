// Package domain holds the webhooks bounded context: outgoing webhook
// subscriptions (endpoints) and their delivery records (#181). Business
// invariants — what a valid endpoint is, SSRF-safe URLs, delivery lifecycle and
// retry/backoff, and payload signing — live here and nowhere else.
package domain

import (
	"errors"
	"strings"
)

// EventType is a domain event a subscriber can listen for. It is a typed enum
// (ubiquitous language) rather than a free string so subscriptions can only ever
// reference events the system actually emits.
type EventType string

const (
	EventLeadCreated          EventType = "lead.created"
	EventLeadQualified        EventType = "lead.qualified"
	EventLeadArchived         EventType = "lead.archived"
	EventPendingReplyApproved EventType = "pending_reply.approved"
	EventSequenceCompleted    EventType = "sequence.completed"
)

// ErrUnknownEventType is returned when a string does not name a known event.
var ErrUnknownEventType = errors.New("webhooks: unknown event type")

// knownEvents is the registry of every event the system can publish. Adding a
// new event means adding it here (and an emit site); subscriptions are validated
// against this set.
var knownEvents = map[EventType]struct{}{
	EventLeadCreated:          {},
	EventLeadQualified:        {},
	EventLeadArchived:         {},
	EventPendingReplyApproved: {},
	EventSequenceCompleted:    {},
}

// KnownEventTypes returns the registered events in a stable order (creation,
// qualification, archival, reply approval, sequence completion) for UI listing.
func KnownEventTypes() []EventType {
	return []EventType{
		EventLeadCreated,
		EventLeadQualified,
		EventLeadArchived,
		EventPendingReplyApproved,
		EventSequenceCompleted,
	}
}

// IsKnown reports whether et is a registered, emittable event.
func (et EventType) IsKnown() bool {
	_, ok := knownEvents[et]
	return ok
}

// ParseEventType normalizes (trim + lowercase) and validates an event name,
// returning ErrUnknownEventType for anything not in the registry.
func ParseEventType(s string) (EventType, error) {
	et := EventType(strings.ToLower(strings.TrimSpace(s)))
	if !et.IsKnown() {
		return "", ErrUnknownEventType
	}
	return et, nil
}
