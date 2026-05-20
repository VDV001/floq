package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/daniil/floq/internal/inbox"
)

// Compile-time check that *channelReplyDispatcher satisfies the inbox
// ReplyDispatcher port.
var _ inbox.ReplyDispatcher = (*channelReplyDispatcher)(nil)

// ErrChannelDispatcherUnsupported signals that a PendingReply arrived
// on a channel the router has no dispatcher for. Distinct sentinel so
// the usecase layer can map it to a clear operator-facing error
// rather than conflating with a transport-layer 5xx.
var ErrChannelDispatcherUnsupported = errors.New("channel reply dispatcher: unsupported channel")

// channelReplyDispatcher routes an approved PendingReply to the
// channel-specific dispatcher (telegram, email, …) based on
// pr.Channel. It is the single ReplyDispatcher the inbox usecase
// holds; channel-aware fanout lives behind this seam so the usecase
// stays oblivious to transport.
//
// Either dispatcher may be nil at construction so the composition
// root can build the router before every transport is initialised;
// a Dispatch call on an unwired channel returns the unsupported
// sentinel so misconfiguration surfaces as a clear error rather than
// a panic.
type channelReplyDispatcher struct {
	telegram inbox.ReplyDispatcher
	email    inbox.ReplyDispatcher
}

func newChannelReplyDispatcher(telegram, email inbox.ReplyDispatcher) *channelReplyDispatcher {
	return &channelReplyDispatcher{telegram: telegram, email: email}
}

// Dispatch — stub for the RED step of #53. Real impl lands in the
// matching GREEN commit; tests fail at runtime so the build stays
// bisect-friendly.
func (d *channelReplyDispatcher) Dispatch(_ context.Context, pr *inbox.PendingReply) error {
	return fmt.Errorf("channelReplyDispatcher: Dispatch not implemented for %q", pr.Channel)
}
